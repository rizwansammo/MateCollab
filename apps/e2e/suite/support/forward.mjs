// Forwards localhost:<port> to <host>:<port> inside the test container, so the browser reaches the
// stack on localhost (where Secure cookies work over plain HTTP) and the Host/Origin stay
// `localhost:8080` as the API's CSRF checks expect.
import net from 'node:net';

const [listenPort, host, targetPort] = process.argv.slice(2);

function forward(client) {
  const upstream = net.connect(Number(targetPort), host);
  client.pipe(upstream).pipe(client);
  const close = () => {
    client.destroy();
    upstream.destroy();
  };
  client.on('error', close);
  upstream.on('error', close);
}

// "localhost" may resolve to either address family.
for (const address of ['127.0.0.1', '::1']) {
  net
    .createServer(forward)
    .on('error', () => {})
    .listen(Number(listenPort), address);
}
