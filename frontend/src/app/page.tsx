import { KotonohaApp } from '@/components/kotonoha-app';
import { PwaRegistrar } from '@/components/pwa-registrar';

export default function HomePage() {
  return (
    <>
      <PwaRegistrar />
      <KotonohaApp />
    </>
  );
}
