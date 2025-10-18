import { useCallback, useState } from "react";
import { getApiBaseUrl, getDefaultApiBaseUrl, setApiBaseUrl } from "../api";

export function useApiBaseUrl() {
  const [baseUrl, setBaseUrlState] = useState(() => getApiBaseUrl());
  const defaultBaseUrl = getDefaultApiBaseUrl();

  const update = useCallback((value: string) => {
    const next = setApiBaseUrl(value);
    setBaseUrlState(next);
    return next;
  }, []);

  const reset = useCallback(() => {
    const next = setApiBaseUrl(null);
    setBaseUrlState(next);
    return next;
  }, []);

  return { baseUrl, defaultBaseUrl, update, reset } as const;
}
