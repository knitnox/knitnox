import { redirect } from "@sveltejs/kit";
async function load({ url }) {
  const id = url.searchParams.get("id");
  if (id) throw redirect(302, `/app/${id}`);
  throw redirect(302, "/apps");
}
export {
  load
};
