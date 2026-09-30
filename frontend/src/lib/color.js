// Deterministic colour per host name, so each host is recognisable across
// cards and tabs. Picked from a fixed set of hues that all read well on the
// dark background.
const HUES = [262, 212, 188, 158, 330, 24, 290, 44, 350, 232];

export function hueFor(text) {
    let hash = 0;
    for (const ch of text) hash = (hash * 31 + ch.codePointAt(0)) >>> 0;
    return HUES[hash % HUES.length];
}
