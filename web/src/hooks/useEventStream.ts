import { useEffect, useState } from 'react';

export type KernelEvent = {
    type: 'status' | 'log';
    data: string;
    timestamp: number;
};

export function useEventStream(jobId: string | null) {
    const [events, setEvents] = useState<KernelEvent[]>([]);
    const [status, setStatus] = useState<'connecting' | 'connected' | 'error' | 'closed'>('closed');

    useEffect(() => {
        if (!jobId) {
            setEvents([]);
            setStatus('closed');
            return;
        }

        setEvents([]);
        setStatus('connecting');
        const eventSource = new EventSource(`http://localhost:8080/v1/jobs/${jobId}/stream`);

        const appendEvent = (type: 'status' | 'log', data: string) => {
            setEvents((prev) => [
                ...prev,
                {
                    type,
                    data,
                    timestamp: Math.floor(Date.now() / 1000),
                },
            ]);
        };

        eventSource.onopen = () => {
            setStatus('connected');
        };

        eventSource.addEventListener('status', (event: MessageEvent) => {
            appendEvent('status', event.data);
        });

        eventSource.addEventListener('log', (event: MessageEvent) => {
            appendEvent('log', event.data);
        });

        eventSource.onmessage = (event) => {
            // Heartbeats or raw messages
            if (event.data === 'ping') return;

            // Fallback for unnamed events
            appendEvent('log', event.data);
        };

        eventSource.onerror = (err) => {
            console.error('SSE Error', err);
            setStatus('error');
            eventSource.close();
        };

        return () => {
            eventSource.close();
            setStatus('closed');
        };
    }, [jobId]);

    return { events, status };
}
