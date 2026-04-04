import { redirect } from '@sveltejs/kit';

/** Backward compatibility: redirect old /app?id=<id> URLs to /app/<id> */
export async function load({ url }) {
  const id = url.searchParams.get('id');
  if (id) throw redirect(302, `/app/${id}`);
  throw redirect(302, '/apps');
}
