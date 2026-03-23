declare module "alpinejs" {
  const Alpine: {
    data: (name: string, callback: () => unknown) => void;
    start: () => void;
  };
  export default Alpine;
}
