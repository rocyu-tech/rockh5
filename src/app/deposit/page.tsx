'use client';

import { Suspense, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

function DepositRedirectInner() {
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const tab = searchParams.get('tab') || 'deposit';
    router.replace('/wallet?tab=' + tab);
  }, [router, searchParams]);

  return null;
}

export default function DepositRedirect() {
  return (
    <Suspense>
      <DepositRedirectInner />
    </Suspense>
  );
}
