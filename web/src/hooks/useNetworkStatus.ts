import { useEffect, useState } from "react";

interface NetworkStatus {
  isOnline: boolean;
  lastChanged: number | null;
  connectionType: string | null;
}

function getIsOnline(): boolean {
  if (typeof navigator === "undefined") {
    return true;
  }
  if (typeof navigator.onLine === "boolean") {
    return navigator.onLine;
  }
  return true;
}

function getConnectionType(): string | null {
  if (typeof navigator === "undefined") {
    return null;
  }
  const connection = (navigator as Navigator & {
    connection?: { effectiveType?: string; downlink?: number };
  }).connection;
  if (!connection) {
    return null;
  }
  const chunks: string[] = [];
  if (connection.effectiveType) {
    chunks.push(connection.effectiveType);
  }
  if (typeof connection.downlink === "number" && connection.downlink > 0) {
    chunks.push(`${connection.downlink.toFixed(1)}Mbps`);
  }
  return chunks.length ? chunks.join(" · ") : null;
}

export function useNetworkStatus(): NetworkStatus {
  const [isOnline, setIsOnline] = useState<boolean>(() => getIsOnline());
  const [lastChanged, setLastChanged] = useState<number | null>(null);
  const [connectionType, setConnectionType] = useState<string | null>(() => getConnectionType());

  useEffect(() => {
    const handleStatusChange = () => {
      setIsOnline(getIsOnline());
      setConnectionType(getConnectionType());
      setLastChanged(Date.now());
    };

    if (typeof window !== "undefined") {
      window.addEventListener("online", handleStatusChange);
      window.addEventListener("offline", handleStatusChange);
    }

    const connection = (navigator as Navigator & {
      connection?: { addEventListener?: (type: string, listener: () => void) => void; removeEventListener?: (type: string, listener: () => void) => void };
    }).connection;

    if (connection?.addEventListener) {
      connection.addEventListener("change", handleStatusChange);
    }

    return () => {
      if (typeof window !== "undefined") {
        window.removeEventListener("online", handleStatusChange);
        window.removeEventListener("offline", handleStatusChange);
      }
      if (connection?.removeEventListener) {
        connection.removeEventListener("change", handleStatusChange);
      }
    };
  }, []);

  return { isOnline, lastChanged, connectionType };
}
