import { redirect } from "@sveltejs/kit";
async function load({ params, fetch }) {
  const res = await fetch("/apps.json");
  const apps = await res.json();
  const app = apps.find((a) => a.id === params.id);
  if (!app) throw redirect(302, "/apps");
  return { app };
}
export {
  load
};
