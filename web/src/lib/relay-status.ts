export type RelayStatus = {
  service: string;
  status: string;
  uptimeSeconds: number;
  rooms: {
    total: number;
    waiting: number;
    active: number;
  };
  limits: {
    maxRooms: number;
    roomTTLSeconds: number;
    reapIntervalSeconds: number;
    maxMessageSizeBytes: number;
  };
};
