import { redirect } from 'next/navigation';

export default async function OrganisationIndex({ params }: { params: Promise<{ org: string }> }) {
  const { org } = await params;
  redirect(`/${encodeURIComponent(org)}/home`);
}
