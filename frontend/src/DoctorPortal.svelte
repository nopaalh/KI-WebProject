<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import DocumentPreview from './DocumentPreview.svelte'
  import type { DocumentCategory } from './patientExamples'
  import { ApiError, apiListDoctorPatientFiles, apiListDoctorPatients, apiUploadDoctorPatientFile, classifyFileCategory, classifyFileKind, formatBytes, type ApiFileRecord, type DoctorPatientSummary } from './api'

  let { user, onUnauthorized }: { user: { id?: number; name: string; username: string }; onUnauthorized?: () => void } = $props()
  type DoctorFileView = { name: string; kind: string; category: DocumentCategory; size: number; source: string; record: ApiFileRecord }
  let patients = $state<DoctorPatientSummary[]>([])
  let loadingPatients = $state(true)
  let selectedId = $state<number | null>(null)
  let search = $state('')
  let tab = $state<'records' | 'documents'>('documents')
  let patientFiles = $state<ApiFileRecord[]>([])
  let loadingFiles = $state(false)
  let uploading = $state(false)
  let notice = $state('')
  let fileError = $state('')
  let detail = $state<DoctorFileView | null>(null)
  let previewDialog = $state<HTMLDialogElement>()
  let activeController: AbortController | null = null
  let activeToken = 0
  const patient = $derived(patients.find(item => item.patient_profile_id === selectedId))
  const filtered = $derived(patients.filter(item => `${item.name} ${item.username} #${item.patient_profile_id}`.toLowerCase().includes(search.trim().toLowerCase())))
  const documents = $derived<DoctorFileView[]>(patientFiles.map(rec => ({
    name: rec.original_name, kind: classifyFileKind(rec.original_name), category: classifyFileCategory(rec.original_name, rec.original_type), size: rec.original_size,
    source: rec.uploaded_by_user_id === patient?.user_id ? 'Pasien' : 'Rumah sakit', record: rec,
  })))

  onDestroy(() => activeController?.abort())

  onMount(() => {
    const controller = new AbortController()
    void loadPatients(controller.signal)
    return () => controller.abort()
  })

  async function loadPatients(signal?: AbortSignal) {
    loadingPatients = true
    try {
      patients = await apiListDoctorPatients(signal)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      notice = 'Gagal memuat daftar pasien dari server.'
    } finally {
      loadingPatients = false
    }
  }

  async function openPatient(profileId: number) {
    selectedId = profileId
    tab = 'documents'
    notice = ''
    fileError = ''
    void loadPatientFiles(profileId)
    await tick()
    document.querySelector<HTMLElement>('.patient-summary h1')?.focus()
  }

  function closePatient() {
    activeController?.abort()
    activeToken += 1
    selectedId = null
    patientFiles = []
    notice = ''
    fileError = ''
  }

  async function loadPatientFiles(profileId: number) {
    activeController?.abort()
    const controller = new AbortController()
    activeController = controller
    const token = ++activeToken
    loadingFiles = true
    patientFiles = []
    try {
      const files = await apiListDoctorPatientFiles(profileId, controller.signal)
      if (token === activeToken && selectedId === profileId) patientFiles = files
    } catch (err) {
      if (token !== activeToken || selectedId !== profileId || (err instanceof DOMException && err.name === 'AbortError')) return
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      fileError = 'Gagal memuat daftar berkas pasien.'
    } finally {
      if (token === activeToken && selectedId === profileId) loadingFiles = false
    }
  }

  async function selectFiles(event: Event) {
    const input = event.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    if (!patient || !file || uploading) return
    const targetId = patient.patient_profile_id
    input.value = ''
    fileError = ''
    notice = ''
    if (file.size > 8 * 1024 * 1024) {
      fileError = 'Ukuran berkas maksimal 8 MiB per berkas.'
      return
    }
    uploading = true
    try {
      const saved = await apiUploadDoctorPatientFile(targetId, file)
      if (selectedId !== targetId) return
      notice = `Berkas "${saved.original_name}" berhasil diunggah untuk ${patient.name}.`
      await loadPatientFiles(targetId)
    } catch (err) {
      if (selectedId !== targetId) return
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      fileError = err instanceof ApiError && err.status === 413 ? 'Ukuran berkas melebihi 8 MiB.' : 'Gagal mengunggah berkas ke profil pasien.'
    } finally {
      uploading = false
    }
  }

  function updateRecord(updated: ApiFileRecord) {
    patientFiles = patientFiles.map(item => item.id === updated.id ? updated : item)
    if (detail?.record.id === updated.id) detail = { ...detail, record: updated }
  }

  async function openDocument(file: DoctorFileView) {
    detail = file
    await tick()
    previewDialog?.showModal()
  }

  function fileIcon(file: DoctorFileView) {
    return file.category === 'Foto' ? 'png' : file.category === 'Video' ? 'mp4' : file.kind === 'PDF' ? 'pdf' : 'docx'
  }
</script>

<svelte:head><title>{patient ? `${patient.name} · Ruang dokter` : 'Pasien · Ruang dokter'}</title></svelte:head>

{#if !patient}
  <main class="patient-page">
    <header class="patient-heading"><p class="eyebrow">Ruang dokter</p><h1 class="patient-title">Pasien Anda.</h1><p class="patient-subtitle">Buka profil pasien untuk melihat atau mengunggah dokumen pemeriksaan.</p></header>
    <div class="overview"><div class="patient-card"><span>Pasien terdaftar</span><strong>{loadingPatients ? '…' : patients.length.toString().padStart(2, '0')}</strong></div><div class="patient-card"><span>Layanan rekam medis</span><strong>Belum tersedia</strong></div><div class="patient-card overview-note"><img src="/images/document-stack.svg" alt="" width="80" height="70" /><p>Setiap pemeriksaan,<br />tercatat dengan jelas.</p></div></div>
    <section class="patient-card patient-list" aria-labelledby="patients-title">
      <div class="list-heading"><div><h2 id="patients-title">Daftar pasien</h2><p>Direktori pasien yang telah membuat profil pasien.</p></div><input type="search" placeholder="Cari nama atau username pasien" aria-label="Cari pasien" bind:value={search} /></div>
      <div class="table-scroll"><table><thead><tr><th>Pasien</th><th>ID Profil</th><th>Username</th><th><span class="action-label">Tindakan</span></th></tr></thead><tbody>{#each filtered as item (item.patient_profile_id)}<tr><td><div class="patient-name"><img class="avatar" src={`/images/patient-${item.patient_profile_id % 2 === 0 ? 'female' : 'male'}.webp`} alt="" width="64" height="64" /><span><strong>{item.name}</strong><small>User #{item.user_id}</small></span></div></td><td>#{item.patient_profile_id}</td><td>@{item.username}</td><td><button class="patient-button secondary" type="button" aria-label={`Buka pasien ${item.name}`} onclick={() => openPatient(item.patient_profile_id)}>Buka pasien</button></td></tr>{:else}<tr><td colspan="4" class="empty"><h3>{loadingPatients ? 'Memuat daftar pasien…' : patients.length === 0 ? 'Belum ada profil pasien terdaftar.' : 'Pasien tidak ditemukan.'}</h3>{#if patients.length > 0}<button class="text-button" type="button" onclick={() => search = ''}>Tampilkan semua pasien</button>{/if}</td></tr>{/each}</tbody></table></div>
    </section>
    {#if notice}<p class="patient-message" role="alert">{notice}</p>{/if}
  </main>
{:else}
  <div class="doctor-workspace">
    <section class="patient-summary" aria-label="Pasien yang sedang dibuka">
      <button class="text-button" type="button" onclick={closePatient}>Semua pasien</button>
      <div class="summary-row"><div class="summary-name"><img class="avatar large" src={`/images/patient-${patient.patient_profile_id % 2 === 0 ? 'female' : 'male'}.webp`} alt="" width="88" height="88" /><div><p class="eyebrow">Profil #{patient.patient_profile_id} · @{patient.username}</p><h1 tabindex="-1">{patient.name}</h1></div></div><dl><div><dt>ID Profil Pasien</dt><dd>#{patient.patient_profile_id}</dd></div><div><dt>ID User</dt><dd>#{patient.user_id}</dd></div></dl></div>
    </section>
    <nav class="doctor-tabs" aria-label="Data pasien">{#each [{ id: 'documents' as const, label: 'Dokumen pasien' }, { id: 'records' as const, label: 'Rekam medis' }] as item}<button type="button" aria-current={tab === item.id ? 'page' : undefined} class:active={tab === item.id} onclick={() => { tab = item.id; notice = ''; fileError = '' }}>{item.label}</button>{/each}</nav>
    {#if tab === 'records'}
      <main class="patient-page"><section class="patient-card empty"><h2>Layanan rekam medis belum tersedia</h2><p>Endpoint backend untuk rekam medis, status draf/final, serta riwayat perubahan belum diimplementasikan. Gunakan tab Dokumen pasien untuk mengunggah laporan pemeriksaan.</p></section></main>
    {:else}
      <main class="patient-page">
        <header class="patient-heading document-heading"><div><h1 class="patient-title">Dokumen pasien</h1><p class="patient-subtitle">Berkas dari pasien dan dokumen pendukung pemeriksaan.</p></div><label class="patient-button file-picker">{uploading ? 'Mengunggah…' : 'Tambah dokumen'}<input type="file" disabled={uploading} onchange={selectFiles} /></label></header>
        {#if fileError}<p class="patient-message" role="alert">{fileError}</p>{/if}
        {#if notice}<p class="patient-message" role="status">{notice}</p>{/if}
        <section class="patient-card document-list" aria-label="Daftar dokumen pasien">{#each documents as file (file.record.id)}<button class="document-row" type="button" onclick={() => openDocument(file)}><img src={`/images/file-${fileIcon(file)}.svg`} alt="" width="46" height="54" /><span class="document-name"><strong>{file.name}</strong><small>{file.kind} · {formatBytes(file.size)} · {file.source}</small></span><span class="file-status">ID #{file.record.id}</span></button>{:else}<div class="empty"><img src="/images/document-stack.svg" alt="" width="100" height="80" /><h2>{loadingFiles ? 'Memuat dokumen…' : 'Belum ada dokumen.'}</h2><p>Tambahkan dokumen pendukung untuk pasien ini.</p></div>{/each}</section>
      </main>
    {/if}
  </div>
{/if}

<dialog bind:this={previewDialog} onclose={() => detail = null} aria-labelledby="doctor-document-title"><form method="dialog"><button class="text-button" type="submit">Tutup</button></form>{#if detail}<h2 id="doctor-document-title">{detail.name}</h2><p class="patient-note">{detail.source} · ID #{detail.record.id}</p><DocumentPreview file={detail} remoteFile={detail.record} onRemoteFileUpdated={updateRecord} {onUnauthorized} />{/if}</dialog>

<style>
  .eyebrow { margin: 0 0 12px; font-size: 11px; font-weight: 500; letter-spacing: .5px; }
  .overview { display: grid; grid-template-columns: 1fr 1fr 1.2fr; gap: 20px; margin-bottom: 32px; }
  .overview > div { padding: 26px 28px; }
  .overview span { font-size: 12px; }
  .overview strong { display: block; font-size: 32px; font-weight: 500; letter-spacing: -1.5px; margin-top: 10px; }
  .overview-note { display: flex; align-items: center; justify-content: center; gap: 22px; background: #f4f8fd; }
  .overview-note p { font-size: 15px; line-height: 1.7; letter-spacing: -.3px; }
  h2 { margin: 0; font-size: 21px; font-weight: 550; letter-spacing: -.5px; }
  .list-heading { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 28px; }
  .list-heading p { margin: 8px 0 0; font-size: 12px; }
  .list-heading input { width: 280px; max-width: 100%; height: 44px; padding: 0 14px; font-size: 12px; }
  .table-scroll { overflow-x: auto; border-radius: 0 0 22px 22px; }
  table { width: 100%; border-collapse: collapse; text-align: left; }
  th { background: #f7f9fb; font-size: 10px; font-weight: 500; padding: 14px 28px; border-block: 1px solid #e1e6ec; white-space: nowrap; }
  td { padding: 24px 28px; font-size: 12px; border-bottom: 1px solid #e1e6ec; white-space: nowrap; }
  tr:last-child td { border-bottom: 0; }
  td:last-child { text-align: right; }
  .action-label { display: block; text-align: right; }
  .patient-name, .summary-name { display: flex; align-items: center; gap: 16px; }
  .patient-name strong { display: block; font-size: 14px; font-weight: 550; }
  small { display: block; margin-top: 5px; font-size: 11px; line-height: 1.7; }
  .avatar { display: block; flex-shrink: 0; width: 64px; height: 64px; object-fit: contain; filter: grayscale(1); }
  .avatar.large { width: 88px; height: 88px; }
  .empty, td.empty { padding: 48px 24px; text-align: center; white-space: normal; }
  .empty h3 { font-size: 17px; font-weight: 550; }
  .empty p { font-size: 12px; }
  .empty img { margin-bottom: 20px; }
  .patient-summary, .doctor-tabs { width: calc(100% - 80px); max-width: 1280px; margin: 0 auto; }
  .patient-summary { padding-top: 28px; }
  .summary-row { display: flex; align-items: center; justify-content: space-between; gap: 30px; padding: 22px 0 30px; }
  .summary-name { min-width: 0; }
  .summary-name .eyebrow { margin-bottom: 7px; }
  .summary-name h1 { font-size: clamp(24px, 3vw, 32px); font-weight: 550; letter-spacing: -1px; margin: 0; overflow-wrap: anywhere; }
  dl { display: flex; gap: 36px; margin: 0; }
  dt { font-size: 10px; margin-bottom: 8px; }
  dd { font-size: 14px; margin: 0; white-space: nowrap; }
  .doctor-tabs { display: flex; gap: 32px; border-bottom: 1px solid #d8dfe6; }
  .doctor-tabs button { padding: 16px 0; border: 0; border-bottom: 2px solid transparent; background: none; font-size: 13px; }
  .doctor-tabs button.active { color: #0069d4; border-bottom-color: #0071e3; }
  .document-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 24px; }
  .file-picker { position: relative; cursor: pointer; }
  .file-picker input { position: absolute; inset: 0; opacity: 0; width: 100%; cursor: pointer; }
  .file-picker:focus-within { outline: 2px solid #0071e3; outline-offset: 4px; }
  .document-list { margin-top: 28px; overflow: hidden; }
  .document-row { width: 100%; display: flex; align-items: center; gap: 20px; border: 0; border-bottom: 1px solid #e1e6ec; padding: 24px 28px; text-align: left; background: #fff; }
  .document-row:last-child { border-bottom: 0; }
  .document-row:hover { background: #f6f9fd; }
  .document-row img { flex-shrink: 0; }
  .document-name { flex: 1; min-width: 0; }
  .document-name strong { font-size: 14px; font-weight: 550; overflow-wrap: anywhere; }
  .file-status { font-size: 10px; background: #f3f6fa; padding: 6px 10px; border: 1px solid #e0e6ee; border-radius: 7px; }
  dialog { width: min(720px, calc(100% - 40px)); max-height: calc(100svh - 40px); padding: 28px; border: 1px solid #d1d8e0; border-radius: 22px; color: #111; }
  dialog::backdrop { background: #111b2b55; backdrop-filter: blur(5px); }
  dialog > form { display: flex; justify-content: flex-end; margin-bottom: 14px; }
  dialog h2 { overflow-wrap: anywhere; }
  @media (max-width: 900px) { .overview { grid-template-columns: 1fr 1fr; } .overview-note { display: none; } .summary-row { align-items: start; flex-direction: column; gap: 24px; } }
  @media (max-width: 680px) { .patient-summary, .doctor-tabs { width: calc(100% - 40px); } .overview { gap: 12px; } .overview > div { padding: 20px; } .overview strong { font-size: 28px; } .list-heading { align-items: stretch; flex-direction: column; padding: 24px; } .list-heading input { width: 100%; } th, td { padding-inline: 22px; } .document-row { padding: 20px; gap: 14px; flex-wrap: wrap; } .file-status { margin-left: 60px; } .document-name strong { font-size: 12px; } dl { gap: 26px; } }
</style>
