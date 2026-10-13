import { redirect } from 'next/navigation';

export default async function AdminIndex({ params }: { params: Promise<{ org: string }> }) {
  const { org } = await params;
  redirect(`/${encodeURIComponent(org)}/admin/general`);
}
