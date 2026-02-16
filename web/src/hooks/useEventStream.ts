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
        if (!jobId) return;

        setStatus('connecting');
        const eventSource = new EventSource(`http://localhost:8080/v1/jobs/${jobId}/stream`);

        eventSource.onopen = () => {
            setStatus('connected');
        };

        eventSource.onmessage = (event) => {
            // Heartbeats or raw messages
            if (event.data === 'ping') return;

            try {
                const parsed = JSON.parse(event.data) as KernelEvent;
                setEvents((prev) => [...prev, parsed]);
            } catch (e) {
                // Fallback for raw text if any
                console.warn('Failed to parse event', event.data);
            }
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
