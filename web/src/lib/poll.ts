import { useCallback, useEffect, useState } from "react";

export function usePoll<T>(fn: () => Promise<T>, ms: number) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const next = await fn();
      setData(next);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [fn]);

  useEffect(() => {
    let stop = false;
    async function tick() {
      try {
        const next = await fn();
        if (!stop) {
          setData(next);
          setError("");
        }
      } catch (err) {
        if (!stop) setError(err instanceof Error ? err.message : "加载失败");
      } finally {
        if (!stop) setLoading(false);
      }
    }
    void tick();
    const id = setInterval(() => void tick(), ms);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, [fn, ms]);

  return { data, error, loading, refresh };
}
