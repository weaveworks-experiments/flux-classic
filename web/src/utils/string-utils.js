export function labels(kv) {
  const s = [];
  Object.keys(kv).forEach(k => {
    s.push(k + '=' + kv[k]);
  });
  return s.join(', ');
}

export function maybeTruncate(id) {
  if (id.length > 12) {
    return id.substr(0, 12) + '...';
  }
  return id;
}
