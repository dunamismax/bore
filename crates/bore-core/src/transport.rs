//! Transport abstraction for bore.
//!
//! Provides a [`Transport`] trait that abstracts over different network
//! transports (WebSocket via relay, direct TCP, in-process test streams).
//! The key type is [`WebSocketTransport`], which bridges WebSocket frames
//! to `AsyncRead`/`AsyncWrite` so that [`SecureChannel`] can operate over
//! any WebSocket connection transparently.
//!
//! # Architecture
//!
//! The relay protocol is simple:
//! - Sender connects to `ws://relay/ws` with no room param → receives room ID as first text message
//! - Receiver connects to `ws://relay/ws?room=ROOM_ID` → relay pairs them
//! - After pairing, relay forwards WebSocket frames bidirectionally (zero-knowledge)
//!
//! This module wraps a WebSocket connection into a pair of `AsyncRead` +
//! `AsyncWrite` halves using an internal buffer that maps between the
//! message-oriented WebSocket protocol and the byte-stream interface that
//! the Noise handshake and SecureChannel expect.

use std::pin::Pin;
use std::task::{Context, Poll};

use futures_util::stream::Stream;
use futures_util::{Sink, StreamExt};
use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
use tokio::net::TcpStream;
use tokio_tungstenite::tungstenite::Message;
use tokio_tungstenite::{MaybeTlsStream, WebSocketStream, connect_async};
use tracing::{debug, instrument};
use url::Url;

use crate::error::TransportError;

// ---------------------------------------------------------------------------
// Transport trait
// ---------------------------------------------------------------------------

/// Abstraction over a bidirectional byte stream transport.
///
/// Implementations provide the read and write halves that [`SecureChannel`]
/// operates over. This allows bore-core to be transport-agnostic: the same
/// crypto and transfer engine work over WebSocket, direct TCP, or in-process
/// test streams.
pub trait Transport {
    /// The readable half of the transport.
    type Read: AsyncRead + Unpin + Send;
    /// The writable half of the transport.
    type Write: AsyncWrite + Unpin + Send;

    /// Split the transport into its read and write halves.
    fn split(self) -> (Self::Read, Self::Write);
}

// ---------------------------------------------------------------------------
// WebSocket transport — connects to the Go relay
// ---------------------------------------------------------------------------

/// A WebSocket connection to the bore relay server.
///
/// Wraps `tokio-tungstenite`'s `WebSocketStream` into `AsyncRead`/`AsyncWrite`
/// halves so that the Noise handshake and SecureChannel can operate over it
/// transparently.
///
/// # Sender flow
///
/// ```text
/// connect_as_sender(relay_url) → (room_id, WsReader, WsWriter)
/// ```
///
/// # Receiver flow
///
/// ```text
/// connect_as_receiver(relay_url, room_id) → (WsReader, WsWriter)
/// ```
pub struct WebSocketTransport {
    reader: WsReader,
    writer: WsWriter,
}

impl Transport for WebSocketTransport {
    type Read = WsReader;
    type Write = WsWriter;

    fn split(self) -> (WsReader, WsWriter) {
        (self.reader, self.writer)
    }
}

/// Connect to the relay as a sender.
///
/// Creates a new room on the relay and returns the room ID along with the
/// transport. The relay sends the room ID as the first text message after
/// the WebSocket connection is established.
#[instrument(skip_all, fields(relay = %relay_url))]
pub async fn connect_as_sender(
    relay_url: &Url,
) -> Result<(String, WebSocketTransport), TransportError> {
    let ws_url = build_ws_url(relay_url, None)?;
    debug!("connecting to relay as sender: {}", ws_url);

    let (ws_stream, _) = connect_async(ws_url.as_str())
        .await
        .map_err(|e| TransportError::ConnectionFailed(format!("WebSocket connect failed: {e}")))?;

    let (write_half, mut read_half) = ws_stream.split();

    // The relay sends the room ID as the first text message.
    let room_id = match read_half.next().await {
        Some(Ok(Message::Text(text))) => text.to_string(),
        Some(Ok(other)) => {
            return Err(TransportError::RelayError(format!(
                "expected text message with room ID, got: {other:?}"
            )));
        }
        Some(Err(e)) => {
            return Err(TransportError::RelayError(format!(
                "failed to read room ID: {e}"
            )));
        }
        None => {
            return Err(TransportError::RelayError(
                "connection closed before room ID received".to_string(),
            ));
        }
    };

    debug!("sender: received room ID: {}", room_id);

    let transport = WebSocketTransport {
        reader: WsReader::new(read_half),
        writer: WsWriter::new(write_half),
    };

    Ok((room_id, transport))
}

/// Connect to the relay as a receiver.
///
/// Joins an existing room on the relay by room ID and returns the transport.
#[instrument(skip_all, fields(relay = %relay_url, room = %room_id))]
pub async fn connect_as_receiver(
    relay_url: &Url,
    room_id: &str,
) -> Result<WebSocketTransport, TransportError> {
    let ws_url = build_ws_url(relay_url, Some(room_id))?;
    debug!("connecting to relay as receiver: {}", ws_url);

    let (ws_stream, _) = connect_async(ws_url.as_str())
        .await
        .map_err(|e| TransportError::ConnectionFailed(format!("WebSocket connect failed: {e}")))?;

    let (write_half, read_half) = ws_stream.split();

    let transport = WebSocketTransport {
        reader: WsReader::new(read_half),
        writer: WsWriter::new(write_half),
    };

    Ok(transport)
}

/// Build the WebSocket URL for connecting to the relay.
///
/// Converts `http://` → `ws://` and `https://` → `wss://`, appends `/ws`,
/// and optionally adds `?room=ROOM_ID`.
fn build_ws_url(relay_url: &Url, room_id: Option<&str>) -> Result<Url, TransportError> {
    let ws_scheme = match relay_url.scheme() {
        "http" | "ws" => "ws",
        "https" | "wss" => "wss",
        other => {
            return Err(TransportError::ConnectionFailed(format!(
                "unsupported relay URL scheme: {other}"
            )));
        }
    };

    let host = relay_url
        .host_str()
        .ok_or_else(|| TransportError::ConnectionFailed("relay URL has no host".to_string()))?;

    let port_part = relay_url
        .port()
        .map(|p| format!(":{p}"))
        .unwrap_or_default();

    let room_query = room_id.map(|id| format!("?room={id}")).unwrap_or_default();

    let url_str = format!("{ws_scheme}://{host}{port_part}/ws{room_query}");

    Url::parse(&url_str).map_err(|e| {
        TransportError::ConnectionFailed(format!("failed to build WebSocket URL: {e}"))
    })
}

// ---------------------------------------------------------------------------
// WsReader — AsyncRead over a WebSocket read half
// ---------------------------------------------------------------------------

/// Adapts the read half of a WebSocket stream to `AsyncRead`.
///
/// WebSocket is message-oriented; `AsyncRead` is byte-stream-oriented.
/// This adapter buffers incoming binary messages and serves them as a
/// contiguous byte stream to the caller.
pub struct WsReader {
    inner: futures_util::stream::SplitStream<WebSocketStream<MaybeTlsStream<TcpStream>>>,
    buffer: Vec<u8>,
    position: usize,
}

impl WsReader {
    fn new(
        inner: futures_util::stream::SplitStream<WebSocketStream<MaybeTlsStream<TcpStream>>>,
    ) -> Self {
        Self {
            inner,
            buffer: Vec::new(),
            position: 0,
        }
    }
}

impl AsyncRead for WsReader {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        // If we have buffered data, serve it first.
        if self.position < self.buffer.len() {
            let remaining = &self.buffer[self.position..];
            let to_copy = remaining.len().min(buf.remaining());
            buf.put_slice(&remaining[..to_copy]);
            self.position += to_copy;

            // If buffer is fully consumed, reset it.
            if self.position >= self.buffer.len() {
                self.buffer.clear();
                self.position = 0;
            }

            return Poll::Ready(Ok(()));
        }

        // Buffer is empty — poll for the next WebSocket message.
        match Pin::new(&mut self.inner).poll_next(cx) {
            Poll::Pending => Poll::Pending,
            Poll::Ready(None) => {
                // Stream closed — signal EOF.
                Poll::Ready(Ok(()))
            }
            Poll::Ready(Some(Ok(msg))) => {
                let data = match msg {
                    Message::Binary(data) => data.to_vec(),
                    Message::Text(text) => text.as_bytes().to_vec(),
                    Message::Close(_) => {
                        // Treat close as EOF.
                        return Poll::Ready(Ok(()));
                    }
                    Message::Ping(_) | Message::Pong(_) => {
                        // Control frames — wake and try again.
                        cx.waker().wake_by_ref();
                        return Poll::Pending;
                    }
                    _ => {
                        // Frame type — wake and try again.
                        cx.waker().wake_by_ref();
                        return Poll::Pending;
                    }
                };

                if data.is_empty() {
                    cx.waker().wake_by_ref();
                    return Poll::Pending;
                }

                let to_copy = data.len().min(buf.remaining());
                buf.put_slice(&data[..to_copy]);

                if to_copy < data.len() {
                    // Buffer the remainder.
                    self.buffer = data;
                    self.position = to_copy;
                }

                Poll::Ready(Ok(()))
            }
            Poll::Ready(Some(Err(e))) => Poll::Ready(Err(std::io::Error::new(
                std::io::ErrorKind::ConnectionReset,
                format!("WebSocket read error: {e}"),
            ))),
        }
    }
}

// ---------------------------------------------------------------------------
// WsWriter — AsyncWrite over a WebSocket write half
// ---------------------------------------------------------------------------

/// Adapts the write half of a WebSocket stream to `AsyncWrite`.
///
/// Each `write()` call sends the data as a single binary WebSocket message.
/// This maps naturally to the length-prefixed framing that SecureChannel
/// and the Noise handshake use.
pub struct WsWriter {
    inner: futures_util::stream::SplitSink<WebSocketStream<MaybeTlsStream<TcpStream>>, Message>,
    /// Pending send future state. We buffer the write and flush on poll_flush
    /// or when the next write arrives.
    pending: Option<Vec<u8>>,
}

impl WsWriter {
    fn new(
        inner: futures_util::stream::SplitSink<WebSocketStream<MaybeTlsStream<TcpStream>>, Message>,
    ) -> Self {
        Self {
            inner,
            pending: None,
        }
    }
}

impl AsyncWrite for WsWriter {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<std::io::Result<usize>> {
        // If there's a pending message, try to flush it first.
        if let Some(data) = self.pending.take() {
            match Pin::new(&mut self.inner).poll_ready(cx) {
                Poll::Ready(Ok(())) => {
                    if let Err(e) =
                        Pin::new(&mut self.inner).start_send(Message::Binary(data.into()))
                    {
                        return Poll::Ready(Err(std::io::Error::new(
                            std::io::ErrorKind::ConnectionReset,
                            format!("WebSocket send error: {e}"),
                        )));
                    }
                }
                Poll::Ready(Err(e)) => {
                    return Poll::Ready(Err(std::io::Error::new(
                        std::io::ErrorKind::ConnectionReset,
                        format!("WebSocket ready error: {e}"),
                    )));
                }
                Poll::Pending => {
                    self.pending = Some(data);
                    return Poll::Pending;
                }
            }
        }

        // Check if the sink is ready for a new message.
        match Pin::new(&mut self.inner).poll_ready(cx) {
            Poll::Ready(Ok(())) => {
                let data = buf.to_vec();
                let len = data.len();
                if let Err(e) = Pin::new(&mut self.inner).start_send(Message::Binary(data.into())) {
                    return Poll::Ready(Err(std::io::Error::new(
                        std::io::ErrorKind::ConnectionReset,
                        format!("WebSocket send error: {e}"),
                    )));
                }
                Poll::Ready(Ok(len))
            }
            Poll::Ready(Err(e)) => Poll::Ready(Err(std::io::Error::new(
                std::io::ErrorKind::ConnectionReset,
                format!("WebSocket ready error: {e}"),
            ))),
            Poll::Pending => Poll::Pending,
        }
    }

    fn poll_flush(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        // Flush any pending message first.
        if let Some(data) = self.pending.take() {
            match Pin::new(&mut self.inner).poll_ready(cx) {
                Poll::Ready(Ok(())) => {
                    if let Err(e) =
                        Pin::new(&mut self.inner).start_send(Message::Binary(data.into()))
                    {
                        return Poll::Ready(Err(std::io::Error::new(
                            std::io::ErrorKind::ConnectionReset,
                            format!("WebSocket send error: {e}"),
                        )));
                    }
                }
                Poll::Ready(Err(e)) => {
                    return Poll::Ready(Err(std::io::Error::new(
                        std::io::ErrorKind::ConnectionReset,
                        format!("WebSocket ready error: {e}"),
                    )));
                }
                Poll::Pending => {
                    self.pending = Some(data);
                    return Poll::Pending;
                }
            }
        }

        // Now flush the underlying sink.
        Pin::new(&mut self.inner).poll_flush(cx).map_err(|e| {
            std::io::Error::new(
                std::io::ErrorKind::ConnectionReset,
                format!("WebSocket flush error: {e}"),
            )
        })
    }

    fn poll_shutdown(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.inner).poll_close(cx).map_err(|e| {
            std::io::Error::new(
                std::io::ErrorKind::ConnectionReset,
                format!("WebSocket close error: {e}"),
            )
        })
    }
}

// ---------------------------------------------------------------------------
// In-process (duplex) transport — for testing
// ---------------------------------------------------------------------------

/// A transport backed by `tokio::io::duplex` for testing.
///
/// This preserves backward compatibility with all existing tests while
/// fitting into the `Transport` trait.
pub struct DuplexTransport {
    reader: tokio::io::ReadHalf<tokio::io::DuplexStream>,
    writer: tokio::io::WriteHalf<tokio::io::DuplexStream>,
}

impl DuplexTransport {
    /// Create a pair of connected in-process transports.
    ///
    /// Data written to one side is readable from the other.
    /// Each `DuplexStream` half is a bidirectional pipe: writes to `a` are
    /// readable from `b` and vice versa. We split each into read/write halves
    /// and give each transport its own stream's halves.
    pub fn pair(buffer_size: usize) -> (Self, Self) {
        let (a, b) = tokio::io::duplex(buffer_size);
        let (a_read, a_write) = tokio::io::split(a);
        let (b_read, b_write) = tokio::io::split(b);
        (
            Self {
                reader: a_read,
                writer: a_write,
            },
            Self {
                reader: b_read,
                writer: b_write,
            },
        )
    }
}

impl Transport for DuplexTransport {
    type Read = tokio::io::ReadHalf<tokio::io::DuplexStream>;
    type Write = tokio::io::WriteHalf<tokio::io::DuplexStream>;

    fn split(self) -> (Self::Read, Self::Write) {
        (self.reader, self.writer)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    #[test]
    fn build_ws_url_http() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let url = build_ws_url(&relay, None).unwrap();
        assert_eq!(url.as_str(), "ws://localhost:8080/ws");
    }

    #[test]
    fn build_ws_url_https() {
        let relay = Url::parse("https://relay.example.com").unwrap();
        let url = build_ws_url(&relay, None).unwrap();
        assert_eq!(url.as_str(), "wss://relay.example.com/ws");
    }

    #[test]
    fn build_ws_url_with_room() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let url = build_ws_url(&relay, Some("abc123")).unwrap();
        assert_eq!(url.as_str(), "ws://localhost:8080/ws?room=abc123");
    }

    #[test]
    fn build_ws_url_ws_scheme() {
        let relay = Url::parse("ws://relay.local:9090").unwrap();
        let url = build_ws_url(&relay, None).unwrap();
        assert_eq!(url.as_str(), "ws://relay.local:9090/ws");
    }

    #[test]
    fn build_ws_url_wss_scheme() {
        let relay = Url::parse("wss://relay.example.com").unwrap();
        let url = build_ws_url(&relay, Some("room1")).unwrap();
        assert_eq!(url.as_str(), "wss://relay.example.com/ws?room=room1");
    }

    #[test]
    fn build_ws_url_rejects_bad_scheme() {
        let relay = Url::parse("ftp://relay.example.com").unwrap();
        let result = build_ws_url(&relay, None);
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn duplex_transport_bidirectional() {
        let (a, b) = DuplexTransport::pair(65536);
        let (mut a_read, mut a_write) = a.split();
        let (mut b_read, mut b_write) = b.split();

        // A writes, B reads
        a_write.write_all(b"hello from A").await.unwrap();
        let mut buf = vec![0u8; 12];
        b_read.read_exact(&mut buf).await.unwrap();
        assert_eq!(&buf, b"hello from A");

        // B writes, A reads
        b_write.write_all(b"hello from B").await.unwrap();
        let mut buf2 = vec![0u8; 12];
        a_read.read_exact(&mut buf2).await.unwrap();
        assert_eq!(&buf2, b"hello from B");
    }

    #[tokio::test]
    async fn duplex_transport_large_data() {
        let (a, b) = DuplexTransport::pair(1024 * 1024);
        let (mut _a_read, mut a_write) = a.split();
        let (mut b_read, mut _b_write) = b.split();

        let data: Vec<u8> = (0..100_000).map(|i| (i % 256) as u8).collect();
        let expected = data.clone();

        let write_handle = tokio::spawn(async move {
            a_write.write_all(&data).await.unwrap();
        });

        let read_handle = tokio::spawn(async move {
            let mut received = vec![0u8; 100_000];
            b_read.read_exact(&mut received).await.unwrap();
            received
        });

        write_handle.await.unwrap();
        let received = read_handle.await.unwrap();
        assert_eq!(received, expected);
    }

    #[tokio::test]
    async fn duplex_transport_with_handshake() {
        use crate::crypto::{HandshakeRole, handshake};

        let (a, b) = DuplexTransport::pair(65536);
        let (mut a_read, mut a_write) = a.split();
        let (mut b_read, mut b_write) = b.split();

        let code = "42-delta-storm-noble";

        let (init_result, resp_result) = tokio::join!(
            handshake(HandshakeRole::Initiator, code, &mut a_read, &mut a_write),
            handshake(HandshakeRole::Responder, code, &mut b_read, &mut b_write),
        );

        let init_ch = init_result.unwrap();
        let resp_ch = resp_result.unwrap();
        assert!(init_ch.is_initiator());
        assert!(!resp_ch.is_initiator());
    }

    #[tokio::test]
    async fn duplex_transport_with_transfer() {
        use crate::crypto::{HandshakeRole, handshake};
        use crate::engine::{receive_data, send_data};

        // Handshake phase
        let (hs_a, hs_b) = DuplexTransport::pair(65536);
        let (mut a_read, mut a_write) = hs_a.split();
        let (mut b_read, mut b_write) = hs_b.split();

        let code = "7-apple-beach-crown";

        let (init_result, resp_result) = tokio::join!(
            handshake(HandshakeRole::Initiator, code, &mut a_read, &mut a_write),
            handshake(HandshakeRole::Responder, code, &mut b_read, &mut b_write),
        );

        let mut sender_ch = init_result.unwrap();
        let mut receiver_ch = resp_result.unwrap();

        // Transfer phase (over a new transport stream)
        let (tx_a, tx_b) = DuplexTransport::pair(4 * 1024 * 1024);
        let (mut _tx_a_read, mut tx_a_write) = tx_a.split();
        let (mut tx_b_read, mut _tx_b_write) = tx_b.split();

        let test_data = b"hello from transport layer!";

        let (send_result, recv_result) = tokio::join!(
            send_data(&mut sender_ch, &mut tx_a_write, "test.txt", test_data),
            receive_data(&mut receiver_ch, &mut tx_b_read),
        );

        let send_res = send_result.unwrap();
        let recv_res = recv_result.unwrap();
        assert_eq!(recv_res.data, test_data);
        assert_eq!(send_res.sha256, recv_res.sha256);
    }
}
