<script lang="ts">
  import { tick } from 'svelte'
  import Dashboard from './Dashboard.svelte'
  import PatientProfile from './PatientProfile.svelte'
  import MedicalRecords from './MedicalRecords.svelte'
  import { exampleDocuments, exampleMedicalRecords } from './patientExamples'

  let { user, onSwitchAccount }: { user: { name: string; username: string; role: 'patient' | 'doctor' }; onSwitchAccount: () => void } = $props()
  type Page = 'documents' | 'profile' | 'records'
  const pages: { id: Page; label: string }[] = [{ id: 'documents', label: 'Dokumen' }, { id: 'profile', label: 'Profil pasien' }, { id: 'records', label: 'Rekam medis' }]
  const notifications = exampleMedicalRecords.slice(0, 2)
  let page = $state<Page>('documents')
  let recordId = $state<number | null>(null)
  let unread = $state(notifications.map(item => item.id))
  let accountMenu = $state<HTMLDetailsElement>()
  let notificationDialog = $state<HTMLDialogElement>()
  let logoutDialog = $state<HTMLDialogElement>()
  let content = $state<HTMLDivElement>()

  async function navigate(next: Page, id: number | null = null) {
    page = next
    if (next === 'records') {
      recordId = null
      await tick()
      recordId = id
    }
    if (accountMenu) accountMenu.open = false
    await tick()
    content?.querySelector<HTMLElement>('div:not([hidden]) > main h1')?.focus({ preventScroll: true })
    window.scrollTo({ top: 0 })
  }

  function openNotification(id: number) {
    unread = unread.filter(item => item !== id)
    notificationDialog?.close()
    void navigate('records', id)
  }
</script>

<svelte:head><title>{pages.find(item => item.id === page)?.label} · Portal Pasien</title></svelte:head>

<header class="topbar">
  <button class="wordmark" type="button" onclick={() => navigate('documents')} aria-label="Portal Pasien, buka dokumen"><img src="/images/portal-mark.svg" alt="" width="34" height="34" /><span>Portal Pasien<span class="wordmark-caption">Ruang kesehatan Anda</span></span></button>
  <div class="portal-actions">
    <button class="notification-button" type="button" onclick={() => { if (accountMenu) accountMenu.open = false; notificationDialog?.showModal() }} aria-label={`Notifikasi, ${unread.length} belum dibaca`}><svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 16h14l-2-3V9a5 5 0 0 0-10 0v4l-2 3Zm5 4h4" /></svg><span class="notification-text">Notifikasi</span>{#if unread.length}<span class="notification-count">{unread.length}</span>{/if}</button>
    <details class="account-menu" bind:this={accountMenu}>
      <summary aria-label="Menu akun"><span>{user.name}</span><small>Akun pasien</small></summary>
      <div class="account-dropdown"><button type="button" onclick={() => navigate('profile')}>Profil pasien</button><button type="button" onclick={onSwitchAccount}>Ganti akun</button><button type="button" onclick={() => { if (accountMenu) accountMenu.open = false; logoutDialog?.showModal() }}>Keluar</button></div>
    </details>
  </div>
</header>

<nav class="patient-navigation" aria-label="Halaman pasien">{#each pages as item}<button type="button" class:active={page === item.id} aria-current={page === item.id ? 'page' : undefined} onclick={() => navigate(item.id)}>{item.label}</button>{/each}</nav>

<div class="portal-content" bind:this={content}>
  <div hidden={page !== 'documents'}><Dashboard {user} files={exampleDocuments} onOpenProfile={() => navigate('profile')} /></div>
  <div hidden={page !== 'profile'}><PatientProfile {user} /></div>
  <div hidden={page !== 'records'}>{#key recordId}<MedicalRecords records={exampleMedicalRecords} bind:selectedId={recordId} />{/key}</div>
</div>

<dialog class="notification-dialog" bind:this={notificationDialog} aria-labelledby="notifications-title">
  <form method="dialog" class="dialog-close"><button class="text-button" type="submit">Tutup</button></form>
  <h2 id="notifications-title">Notifikasi</h2><p class="patient-note">Pembaruan contoh untuk memperlihatkan alur pemeriksaan.</p>
  <div class="notification-toolbar"><span role="status">{unread.length ? `${unread.length} belum dibaca` : 'Semua pembaruan sudah dibaca'}</span><button class="text-button" type="button" disabled={!unread.length} onclick={() => unread = []}>Tandai semua dibaca</button></div>
  <ul class="notification-list">{#each notifications as item}<li><button type="button" class:unread={unread.includes(item.id)} aria-label={`Buka pembaruan ${item.title}`} onclick={() => openNotification(item.id)}><span class="notification-dot" aria-hidden="true"></span><span><strong>{item.title}</strong><span>{item.versions[0]?.summary}</span><small>{item.versions[0]?.date}</small></span></button></li>{/each}</ul>
</dialog>

<dialog class="logout-dialog" bind:this={logoutDialog} aria-labelledby="logout-title"><img src="/images/portal-mark.svg" alt="" width="40" height="40" /><h2 id="logout-title">Keluar dari akun?</h2><p>Fitur keluar belum tersedia pada tampilan ini. Anda tetap dapat mengganti akun melalui menu akun.</p><div class="logout-actions"><form method="dialog"><button class="patient-button secondary" type="submit">Batal</button></form><button class="patient-button" type="button" disabled>Keluar</button></div></dialog>

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
