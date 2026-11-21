// The address of the server a probe drives, which the harness started for it
// and says in YOMIHON_BASE. There is no default: a probe may write, and a fixed
// address is whatever else happens to be listening there.
export const baseUrl = process.env.YOMIHON_BASE;
if (!baseUrl) {
  console.error('set YOMIHON_BASE to the running server');
  process.exit(2);
}
