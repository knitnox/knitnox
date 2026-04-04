export async function load({ fetch }) {
  const res = await fetch('/apps.json');
  const apps = await res.json();
  return { apps };
}
