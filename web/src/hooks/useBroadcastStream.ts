import { useEffect, useState, useCallback, useRef } from 'react';

export type BroadcastEvent = {
    type: string;
    data: Record<string, unknown>;
    timestamp: number;
};

/**
 * useBroadcastStream connects to the global SSE broadcast endpoint (/v1/events).
 * Receives proactive agent messages from heartbeat, cron, spawn, and other
 * background agent activity — without needing to know job/conversation IDs.
 */
export function useBroadcastStream() {
    const [events, setEvents] = useState<BroadcastEvent[]>([]);
    const [status, setStatus] = useState<'connecting' | 'connected' | 'error' | 'closed'>('closed');
    const eventSourceRef = useRef<EventSource | null>(null);

    const connect = useCallback(() => {
        if (eventSourceRef.current) {
            eventSourceRef.current.close();
        }

        setStatus('connecting');
        const es = new EventSource('http://localhost:8080/v1/events');
        eventSourceRef.current = es;

        es.onopen = () => {
            setStatus('connected');
        };

        es.addEventListener('connected', () => {
            setStatus('connected');
        });

        es.addEventListener('new_message', (event: MessageEvent) => {
            try {
                const data = JSON.parse(event.data);
                setEvents((prev) => [
                    ...prev,
                    {
                        type: data.type || 'message',
                        data,
                        timestamp: data.timestamp || Date.now(),
                    },
                ]);
            } catch {
                console.warn('Failed to parse broadcast event:', event.data);
            }
        });

        es.addEventListener('status', (event: MessageEvent) => {
            try {
                const data = JSON.parse(event.data);
                setEvents((prev) => [
                    ...prev,
                    {
                        type: 'status',
                        data,
                        timestamp: Date.now(),
                    },
                ]);
            } catch {
                // ignore
            }
        });

        es.onerror = () => {
            setStatus('error');
            es.close();
            // Auto-reconnect after 5 seconds
            setTimeout(() => {
                if (eventSourceRef.current === es) {
                    connect();
                }
            }, 5000);
        };
    }, []);

    const disconnect = useCallback(() => {
        if (eventSourceRef.current) {
            eventSourceRef.current.close();
            eventSourceRef.current = null;
        }
        setStatus('closed');
    }, []);

    const clearEvents = useCallback(() => {
        setEvents([]);
    }, []);

    useEffect(() => {
        connect();
        return () => {
            disconnect();
        };
    }, [connect, disconnect]);

    return { events, status, clearEvents, disconnect, connect };
}
