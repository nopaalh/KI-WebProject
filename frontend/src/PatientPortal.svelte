<script lang="ts">
  import { tick } from 'svelte'
  import Dashboard from './Dashboard.svelte'
  import PatientProfile from './PatientProfile.svelte'
  import MedicalRecords from './MedicalRecords.svelte'
  import { exampleDocuments, exampleMedicalRecords } from './patientExamples'

  let { user, onSwitchAccount }: { user: { name: string; username: string; role: 'patient' | 'doctor' }; onSwitchAccount: () => void } = $props()
  type Page = 'documents' | 'profile' | 'records'
  const pages: { id: Page; label: string }[] = [{ id: 'documents', label: 'Dokumen' }, { id: 'profile', label: 'Profil pasien' }, { id: 'records', label: 'Rekam medis' }]
  let page = $state<Page>('documents')
  let recordId = $state<number | null>(null)
  let content = $state<HTMLDivElement>()

  async function navigate(next: Page, id: number | null = null) {
    page = next
    if (next === 'records') {
      recordId = null
      await tick()
      recordId = id
    }
    await tick()
    content?.querySelector<HTMLElement>('div:not([hidden]) > main h1')?.focus({ preventScroll: true })
    window.scrollTo({ top: 0 })
  }

</script>

<svelte:head><title>{pages.find(item => item.id === page)?.label} · Portal Pasien</title></svelte:head>

<header class="topbar">
  <button class="wordmark" type="button" onclick={() => navigate('documents')} aria-label="Portal Pasien, buka dokumen"><img src="/images/portal-mark.svg" alt="" width="34" height="34" /><span>Portal Pasien<span class="wordmark-caption">Ruang kesehatan Anda</span></span></button>
  <div class="portal-actions">
<span class="account-hint">{user.name}</span><button class="back" type="button" onclick={onSwitchAccount}>Ganti akun</button>
  </div>
</header>

<nav class="patient-navigation" aria-label="Halaman pasien">{#each pages as item}<button type="button" class:active={page === item.id} aria-current={page === item.id ? 'page' : undefined} onclick={() => navigate(item.id)}>{item.label}</button>{/each}</nav>

<div class="portal-content" bind:this={content}>
  <div hidden={page !== 'documents'}><Dashboard {user} files={exampleDocuments} onOpenProfile={() => navigate('profile')} /></div>
  <div hidden={page !== 'profile'}><PatientProfile {user} /></div>
  <div hidden={page !== 'records'}>{#key recordId}<MedicalRecords records={exampleMedicalRecords} bind:selectedId={recordId} />{/key}</div>
</div>



<style>
  .portal-actions { display: flex; align-items: center; gap: 20px; margin-left: auto; }
  .notification-button { min-height: 44px; display: flex; align-items: center; gap: 9px; padding: 8px; border: 0; background: none; font-size: 12px; }
  .notification-count { display: grid; place-items: center; min-width: 19px; height: 19px; border-radius: 6px; background: #0071e3; color: #fff; font-size: 10px; }
  .account-menu { position: relative; }
  summary { padding: 5px 0 5px 20px; border-left: 1px solid #d8dfe6; cursor: pointer; list-style: none; max-width: 190px; }
  summary::-webkit-details-marker { display: none; }
  summary > span { display: block; font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
  summary > small { display: block; font-size: 10px; margin-top: 3px; }
  summary:focus-visible { outline: 2px solid #0071e3; outline-offset: 7px; border-radius: 6px; }
  .account-dropdown { position: absolute; z-index: 2; right: 0; top: calc(100% + 12px); width: 190px; padding: 7px; border: 1px solid #d1d8e0; border-radius: 14px; background: #fff; box-shadow: 0 12px 40px #15213a12; }
  .account-dropdown button { width: 100%; min-height: 42px; text-align: left; padding: 10px 12px; border: 0; background: none; border-radius: 8px; font-size: 12px; }
  .account-dropdown button:hover { background: #f1f6ff; }
  .patient-navigation { display: flex; gap: 30px; width: calc(100% - 80px); max-width: 1280px; margin: 0 auto; border-bottom: 1px solid #e1e6ec; }
  .patient-navigation button { position: relative; min-height: 57px; padding: 16px 0; border: 0; background: none; font-size: 13px; white-space: nowrap; }
  .patient-navigation button.active { color: #0071e3; font-weight: 500; }
  .patient-navigation button.active::after { content: ''; position: absolute; height: 2px; border-radius: 2px; background: #0071e3; bottom: -1px; left: 0; right: 0; }
  .portal-content { flex: 1; }
  dialog { width: calc(100% - 40px); max-width: 480px; max-height: calc(100svh - 40px); border: 1px solid #d1d8e0; border-radius: 22px; padding: 28px; color: #111; background: #fff; box-shadow: 0 24px 100px #101e3826; }
  dialog::backdrop { background: #111b3038; backdrop-filter: blur(5px); }
  h2 { font-size: 25px; font-weight: 550; letter-spacing: -.6px; margin: 0; }
  .dialog-close { display: flex; justify-content: flex-end; margin-bottom: 16px; }
  .notification-toolbar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; margin: 26px 0 12px; font-size: 11px; }
  .notification-list { list-style: none; margin: 0; padding: 0; }
  .notification-list li + li { border-top: 1px solid #e1e6ec; }
  .notification-list button { display: flex; gap: 12px; width: 100%; padding: 22px 4px; border: 0; background: none; text-align: left; }
  .notification-dot { flex: none; width: 6px; height: 6px; border-radius: 50%; margin-top: 6px; background: #d4dce7; }
  .unread .notification-dot { background: #0071e3; }
  .notification-list strong { font-size: 14px; font-weight: 550; }
  .notification-list button > span > span { display: block; margin-top: 8px; font-size: 12px; line-height: 1.8; }
  .notification-list small { display: block; margin-top: 10px; font-size: 10px; }
  .logout-dialog img { margin-bottom: 22px; }
  .logout-dialog > p { font-size: 13px; line-height: 1.8; margin: 18px 0 26px; }
  .logout-actions { display: flex; gap: 12px; justify-content: flex-end; }
  @media (max-width: 680px) { .patient-navigation { width: calc(100% - 40px); gap: 25px; } .portal-actions { gap: 12px; } summary { padding-left: 12px; max-width: 140px; } .notification-text { display: none; } .patient-navigation button { font-size: 12px; } }
  @media (max-width: 400px) { .topbar { align-items: center; } .portal-actions { width: 100%; justify-content: space-between; } summary { border-left: 0; padding-left: 0; max-width: 210px; text-align: right; } .patient-navigation { gap: 0; justify-content: space-between; } dialog { padding: 22px; } }
</style>
