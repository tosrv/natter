export interface SocketOpts {
  url: string;
  token: () => string | null;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  onFrame: (f: any) => void;
  onOpen: () => void;
  onClose: (code: number) => void;
  WS?: typeof WebSocket;
  random?: () => number;
}

export function createSocket(opts: SocketOpts): { send(frame: object): boolean; close(): void } {
  const WS = opts.WS ?? WebSocket;
  const random = opts.random ?? Math.random;
  let ws: WebSocket | null = null;
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  const schedule = () => {
    const delay = Math.min(30000, 1000 * 2 ** attempt) * (0.5 + random() / 2);
    attempt++;
    timer = setTimeout(connect, delay);
  };

  function connect() {
    timer = null;
    if (closed) return;
    const tok = opts.token();
    if (!tok) { schedule(); return; }
    let sock: WebSocket;
    try { sock = new WS(opts.url, ["natter.bearer", tok]); } catch { schedule(); return; }
    ws = sock;
    sock.onopen = () => { attempt = 0; opts.onOpen(); };
    sock.onmessage = (ev) => {
      let f: unknown;
      try { f = JSON.parse(String(ev.data)); } catch { return; }
      opts.onFrame(f);
    };
    sock.onclose = (ev) => {
      if (ws !== sock) return;
      ws = null;
      opts.onClose(ev.code);
      if (!closed) schedule();
    };
  }

  connect();

  return {
    send(frame) {
      if (!ws || ws.readyState !== WS.OPEN) return false;
      ws.send(JSON.stringify(frame));
      return true;
    },
    close() {
      closed = true;
      if (timer) clearTimeout(timer);
      timer = null;
      const s = ws;
      ws = null;
      s?.close();
    },
  };
}
