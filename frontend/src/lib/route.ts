import { useEffect, useState } from "react";

export type Route = {
  path: string;
  params: URLSearchParams;
};

function parseHash(): Route {
  const raw = window.location.hash.replace(/^#/, "") || "/";
  const [pathname, query = ""] = raw.split("?");
  return { path: pathname || "/", params: new URLSearchParams(query) };
}

export function navigate(path: string, params?: Record<string, string>) {
  const search = params ? new URLSearchParams(params).toString() : "";
  window.location.hash = search ? `${path}?${search}` : path;
}

export function useRoute(): Route {
  const [route, setRoute] = useState<Route>(parseHash);
  useEffect(() => {
    const onChange = () => setRoute(parseHash());
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return route;
}

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const media = window.matchMedia(query);
    const onChange = () => setMatches(media.matches);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [query]);
  return matches;
}
