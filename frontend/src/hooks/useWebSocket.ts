import { useState, useEffect, useRef, useCallback } from 'react';
import type { MessageRecord } from '@/lib/api';

type ConnectionState = 'disconnected' | 'connecting' | 'connected';

export function useWebSocket(url: string | null) {
  const [messages, setMessages] = useState<MessageRecord[]>([]);
  const [state, setState] = useState<ConnectionState>('disconnected');
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback((filter?: string) => {
    if (!url || wsRef.current) return;

    setState('connecting');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const filterQs = filter ? `?filter=${encodeURIComponent(filter)}` : '';
    const wsUrl = `${protocol}//${window.location.host}${url}${filterQs}`;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      setState('connected');
      ws.send(JSON.stringify({ action: 'start' }));
    };

    ws.onmessage = (event) => {
      const msg: MessageRecord = JSON.parse(event.data);
      setMessages((prev) => [msg, ...prev].slice(0, 1000));
    };

    ws.onclose = () => {
      setState('disconnected');
      wsRef.current = null;
    };

    ws.onerror = () => {
      ws.close();
    };

    wsRef.current = ws;
  }, [url]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.send(JSON.stringify({ action: 'stop' }));
      wsRef.current.close();
      wsRef.current = null;
    }
    setState('disconnected');
  }, []);

  const clear = useCallback(() => {
    setMessages([]);
  }, []);

  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  return { messages, state, connect, disconnect, clear };
}
