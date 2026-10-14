import { createCaptureServer } from './server.js';
const server = createCaptureServer(process.env.CAPTURE_TOKEN ?? '', process.env.CAPTURE_EGRESS_URL ?? '');
server.requestTimeout = 10_000;
server.headersTimeout = 5_000;
server.listen(3001, '0.0.0.0', () => console.log(JSON.stringify({ service: 'capture', port: 3001, captureEnabled: !!process.env.CAPTURE_EGRESS_URL })));
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => {
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 10_000).unref();
});
