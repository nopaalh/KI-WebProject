<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import DocumentPreview from './DocumentPreview.svelte'
  import type { DocumentCategory } from './patientExamples'
  import {
    ApiError,
    apiListDoctorPatientFiles,
    apiListDoctorPatients,
    apiUploadDoctorPatientFile,
    classifyFileCategory,
    classifyFileKind,
    formatBytes,
    type ApiFileRecord,
    type CurrentUser,
    type DoctorPatientSummary,
  } from './api'

  let {
    user,
    onUnauthorized,
  }: {
    user: CurrentUser
    onUnauthorized?: () => void
  } = $props()

  type DoctorFileView = {
    name: string
    kind: string
    category: DocumentCategory
    size: number
    source: string
    record: ApiFileRecord
  }

  let patients = $state<DoctorPatientSummary[]>([])
  let loadingPatients = $state(true)
  let patientsError = $state('')
  let selectedProfileId = $state<number | null>(null)
  let search = $state('')
  let tab = $state<'documents' | 'records'>('documents')

  let patientFiles = $state<ApiFileRecord[]>([])
  let loadingFiles = $state(false)
  let filesError = $state('')
  let selectedUploadFile = $state<File | null>(null)
  let uploading = $state(false)
  let notice = $state('')
  let fileError = $state('')
  let fileInput = $state<HTMLInputElement>()

  let detail = $state<DoctorFileView | null>(null)
  let previewDialog = $state<HTMLDialogElement>()

  let activePatientController: AbortController | null = null
  let activeRequestToken = 0

  const patient = $derived(
    patients.find(item => item.patient_profile_id === selectedProfileId) ?? null,
  )

  const filtered = $derived(
    patients.filter(item =>
      `${item.name} ${item.username} #${item.patient_profile_id}`
        .toLowerCase()
        .includes(search.trim().toLowerCase()),
    ),
  )

  const documentViews = $derived<DoctorFileView[]>(
    patientFiles.map(rec => ({
      name: rec.original_name,
      kind: classifyFileKind(rec.original_name),
      category: classifyFileCategory(rec.original_name, rec.original_type),
      size: rec.original_size,
      source:
        rec.uploaded_by_user_id === patient?.user_id
          ? 'Diunggah pasien'
          : rec.uploaded_by_user_id === user.id
            ? 'Diunggah oleh Anda (Dokter)'
            : 'Diunggah oleh rumah sakit / dokter',
      record: rec,
    })),
  )

  onDestroy(() => {
    activePatientController?.abort()
  })

  onMount(() => {
    const controller = new AbortController()
    void loadPatients(controller.signal)
    return () => controller.abort()
  })

  async function loadPatients(signal?: AbortSignal) {
    loadingPatients = true
    patientsError = ''
    try {
      const list = await apiListDoctorPatients(signal)
      patients = list
      if (
        selectedProfileId !== null &&
        !list.some(item => item.patient_profile_id === selectedProfileId)
      ) {
        closePatient()
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          patientsError = 'Akses ditolak (403). Hanya akun dokter yang dapat membaca direktori pasien.'
        } else if (err.isNetworkError) {
          patientsError = 'Tidak dapat terhubung ke server untuk memuat daftar pasien.'
        } else {
          patientsError = 'Gagal memuat daftar pasien dari server.'
        }
      } else {
        patientsError = 'Terjadi kesalahan saat memuat daftar pasien.'
      }
    } finally {
      loadingPatients = false
    }
  }

  async function openPatient(profileId: number) {
    selectedProfileId = profileId
    tab = 'documents'
    notice = ''
    fileError = ''
    selectedUploadFile = null
    if (fileInput) fileInput.value = ''
    void loadFilesForPatient(profileId)
    await tick()
    document.querySelector<HTMLElement>('.patient-summary h1')?.focus()
  }

  function closePatient() {
    activePatientController?.abort()
    activePatientController = null
    activeRequestToken += 1
    selectedProfileId = null
    patientFiles = []
    filesError = ''
    notice = ''
    fileError = ''
    selectedUploadFile = null
    if (fileInput) fileInput.value = ''
  }

  async function loadFilesForPatient(profileId: number) {
    activePatientController?.abort()
    const controller = new AbortController()
    activePatientController = controller
    const token = ++activeRequestToken

    loadingFiles = true
    filesError = ''
    patientFiles = []

    try {
      const files = await apiListDoctorPatientFiles(profileId, controller.signal)
      if (token !== activeRequestToken || selectedProfileId !== profileId) return
      patientFiles = files
    } catch (err) {
      if (token !== activeRequestToken || selectedProfileId !== profileId) return
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          filesError = 'Akses ditolak (403). Hanya dokter yang dapat mengakses berkas pasien.'
        } else if (err.status === 404) {
          filesError = 'Profil pasien tidak ditemukan di server (404).'
        } else if (err.isNetworkError) {
          filesError = 'Tidak dapat terhubung ke server saat memuat berkas pasien.'
        } else {
          filesError = 'Gagal memuat daftar berkas pasien dari server.'
        }
      } else {
        filesError = 'Terjadi kesalahan saat memuat berkas pasien.'
      }
    } finally {
      if (token === activeRequestToken && selectedProfileId === profileId) {
        loadingFiles = false
      }
    }
  }

  function selectFile(event: Event) {
    if (uploading || !patient) return
    const input = event.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return
    fileError = ''
    notice = ''
    if (file.size > 8 * 1024 * 1024) {
      fileError = 'Ukuran berkas maksimal 8 MiB per berkas.'
      input.value = ''
      selectedUploadFile = null
      return
    }
    selectedUploadFile = file
    notice = `Berkas "${file.name}" (${formatBytes(file.size)}) siap diunggah untuk ${patient.name}.`
  }

  function cancelSelectedFile() {
    if (uploading) return
    selectedUploadFile = null
    notice = ''
    fileError = ''
    if (fileInput) fileInput.value = ''
  }

  async function submitPatientFile() {
    if (!patient || !selectedUploadFile || uploading) return
    const targetProfileId = patient.patient_profile_id
    const targetName = patient.name
    const fileToUpload = selectedUploadFile

    uploading = true
    fileError = ''
    notice = ''
    try {
      const saved = await apiUploadDoctorPatientFile(targetProfileId, fileToUpload)
      if (selectedProfileId !== targetProfileId) return
      selectedUploadFile = null
      if (fileInput) fileInput.value = ''
      notice = `Berkas "${saved.original_name}" berhasil diunggah untuk ${targetName}.`
      await loadFilesForPatient(targetProfileId)
    } catch (err) {
      if (selectedProfileId !== targetProfileId) return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          fileError = 'Akses ditolak (403). Hanya akun dokter yang dapat mengunggah berkas pasien.'
        } else if (err.status === 404) {
          fileError = 'Profil pasien target tidak ditemukan (404).'
        } else if (err.status === 413) {
          fileError = 'Ukuran berkas melebihi batas maksimum 8 MiB.'
        } else if (err.status === 400) {
          fileError = 'Permintaan unggah berkas tidak valid.'
        } else if (err.isNetworkError) {
          fileError = 'Tidak dapat terhubung ke server saat mengunggah berkas.'
        } else {
          fileError = 'Gagal memproses dan menyimpan berkas di server.'
        }
      } else {
        fileError = 'Terjadi kesalahan saat mengunggah berkas.'
      }
    } finally {
      uploading = false
    }
  }

  function handleRemoteUpdated(updated: ApiFileRecord) {
    patientFiles = patientFiles.map(item => (item.id === updated.id ? updated : item))
    if (detail && detail.record.id === updated.id) {
      detail = { ...detail, record: updated }
    }
  }

  async function openDocument(file: DoctorFileView) {
    detail = file
    await tick()
    previewDialog?.showModal()
  }

  function fileIcon(file: DoctorFileView) {
    return file.category === 'Foto'
      ? 'png'
      : file.category === 'Video'
        ? 'mp4'
        : file.kind === 'PDF'
          ? 'pdf'
          : 'docx'
  }

  function avatarFor(profileId: number): 'male' | 'female' {
    return profileId % 2 === 0 ? 'female' : 'male'
  }
</script>

<svelte:head><title>{patient ? `${patient.name} · Ruang dokter` : 'Pasien · Ruang dokter'}</title></svelte:head>

{#if !patient}
  <main class="patient-page">
    <header class="patient-heading">
      <p class="eyebrow">Ruang dokter</p>
      <h1 class="patient-title">Pasien Anda.</h1>
      <p class="patient-subtitle">Pilih profil pasien terdaftar untuk melihat atau mengunggah dokumen pemeriksaan.</p>
    </header>

    <div class="overview">
      <div class="patient-card">
        <span>Profil pasien terdaftar</span>
        <strong>{loadingPatients ? '…' : patients.length.toString().padStart(2, '0')}</strong>
      </div>
      <div class="patient-card">
        <span>Rekam medis rumah sakit</span>
        <strong>Belum tersedia</strong>
        <small>Endpoint backend rekam medis belum diimplementasikan</small>
      </div>
      <div class="patient-card overview-note">
        <img src="/images/document-stack.svg" alt="" width="80" height="70" />
        <p>Direktori pasien & lampiran<br />terhubung langsung ke server.</p>
      </div>
    </div>

    <section class="patient-card patient-list" aria-labelledby="patients-title">
      <div class="list-heading">
        <div>
          <h2 id="patients-title">Daftar pasien</h2>
          <p>Menampilkan akun pasien yang telah membuat profil pasien (`GET /api/doctor/patients`).</p>
        </div>
        <div class="list-controls">
          <input type="search" placeholder="Cari nama, username, atau ID profil" aria-label="Cari pasien" bind:value={search} />
          <button class="patient-button secondary" type="button" disabled={loadingPatients} onclick={() => loadPatients()}>
            {loadingPatients ? 'Memuat…' : 'Muat ulang'}
          </button>
        </div>
      </div>

      {#if patientsError}
        <div class="error-banner" role="alert">
          <p>{patientsError}</p>
          <button class="patient-button secondary" type="button" onclick={() => loadPatients()}>Coba lagi</button>
        </div>
      {/if}

      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Pasien</th>
              <th>ID Profil Pasien</th>
              <th>Username Akun</th>
              <th><span class="action-label">Tindakan</span></th>
            </tr>
          </thead>
          <tbody>
            {#if loadingPatients && !patients.length}
              <tr><td colspan="4" class="empty"><h3>Memuat daftar pasien dari server…</h3></td></tr>
            {:else}
              {#each filtered as item (item.patient_profile_id)}
                <tr>
                  <td>
                    <div class="patient-name">
                      <img class="avatar" src={`/images/patient-${avatarFor(item.patient_profile_id)}.webp`} alt="" width="64" height="64" />
                      <span>
                        <strong>{item.name}</strong>
                        <small>User ID #{item.user_id}</small>
                      </span>
                    </div>
                  </td>
                  <td><code>#{item.patient_profile_id}</code></td>
                  <td>@{item.username}</td>
                  <td>
                    <button class="patient-button secondary" type="button" aria-label={`Buka pasien ${item.name}`} onclick={() => openPatient(item.patient_profile_id)}>
                      Buka pasien
                    </button>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="empty">
                    {#if patients.length === 0}
                      <h3>Belum ada profil pasien terdaftar.</h3>
                      <p>Pasien perlu masuk dan membuat profil pasien terlebih dahulu agar muncul di direktori dokter.</p>
                    {:else}
                      <h3>Pasien tidak ditemukan.</h3>
                      <p>Coba kata kunci nama, username, atau ID profil lain.</p>
                      <button class="text-button" type="button" onclick={() => search = ''}>Tampilkan semua pasien</button>
                    {/if}
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </section>
  </main>
{:else}
  <div class="doctor-workspace">
    <section class="patient-summary" aria-label="Pasien yang sedang dibuka">
      <button class="text-button" type="button" onclick={closePatient}>← Semua pasien</button>
      <div class="summary-row">
        <div class="summary-name">
          <img class="avatar large" src={`/images/patient-${avatarFor(patient.patient_profile_id)}.webp`} alt="" width="88" height="88" />
          <div>
            <p class="eyebrow">Profil Pasien #{patient.patient_profile_id} · @{patient.username}</p>
            <h1 tabindex="-1">{patient.name}</h1>
          </div>
        </div>
        <dl>
          <div><dt>ID Profil Pasien</dt><dd>#{patient.patient_profile_id}</dd></div>
          <div><dt>ID Akun User</dt><dd>#{patient.user_id}</dd></div>
          <div><dt>Detail Biometrik & KTP</dt><dd>Tidak diekspos di endpoint dokter</dd></div>
        </dl>
      </div>
    </section>

    <nav class="doctor-tabs" aria-label="Data pasien">
      {#each [{ id: 'documents' as const, label: 'Dokumen & Lampiran Pasien' }, { id: 'records' as const, label: 'Rekam Medis (Belum Tersedia)' }] as item}
        <button type="button" aria-current={tab === item.id ? 'page' : undefined} class:active={tab === item.id} onclick={() => { tab = item.id; notice = ''; fileError = '' }}>
          {item.label}
        </button>
      {/each}
    </nav>

    {#if tab === 'documents'}
      <main class="patient-page">
        <header class="patient-heading document-heading">
          <div>
            <h1 class="patient-title">Dokumen pasien</h1>
            <p class="patient-subtitle">Lampiran milik profil pasien #{patient.patient_profile_id} (`GET /api/doctor/patients/{patient.patient_profile_id}/files`).</p>
          </div>
          <div class="document-actions-bar">
            <button class="patient-button secondary" type="button" disabled={loadingFiles} onclick={() => loadFilesForPatient(patient.patient_profile_id)}>
              {loadingFiles ? 'Memuat…' : 'Muat ulang'}
            </button>
            <label class="patient-button file-picker">
              Pilih laporan / lampiran
              <input type="file" bind:this={fileInput} disabled={uploading} onchange={selectFile} />
            </label>
          </div>
        </header>

        {#if selectedUploadFile}
          <div class="patient-card pending-upload-bar">
            <div>
              <strong>Siap diunggah:</strong> {selectedUploadFile.name} ({formatBytes(selectedUploadFile.size)})
              <small>Berkas akan disimpan dengan tiga representasi cipher (AES, DES, RC4) untuk profil pasien #{patient.patient_profile_id}.</small>
            </div>
            <div class="pending-upload-actions">
              <button class="patient-button" type="button" disabled={uploading} onclick={submitPatientFile}>
                {uploading ? 'Mengunggah…' : 'Unggah ke profil pasien'}
              </button>
              <button class="patient-button secondary" type="button" disabled={uploading} onclick={cancelSelectedFile}>
                Batal
              </button>
            </div>
          </div>
        {/if}

        {#if fileError}<p class="patient-message error" role="alert">{fileError}</p>{/if}
        {#if filesError}<p class="patient-message error" role="alert">{filesError}</p>{/if}
        {#if notice}<p class="patient-message" role="status">{notice}</p>{/if}

        <section class="patient-card document-list" aria-label="Daftar dokumen pasien">
          {#if loadingFiles && !documentViews.length}
            <div class="empty">
              <h2>Memuat lampiran pasien…</h2>
              <p>Mengambil daftar berkas dari server.</p>
            </div>
          {:else}
            {#each documentViews as file (file.record.id)}
              <button class="document-row" type="button" onclick={() => openDocument(file)}>
                <img src={`/images/file-${fileIcon(file)}.svg`} alt="" width="46" height="54" />
                <span class="document-name">
                  <strong>{file.name}</strong>
                  <small>
                    {file.kind} · {formatBytes(file.size)} · {file.source} · Cipher: {(file.record.algorithms?.length ? file.record.algorithms : ['aes']).map(a => a.toUpperCase()).join('/')}
                  </small>
                </span>
                <span class="file-status">ID #{file.record.id} · Lihat / Unduh</span>
              </button>
            {:else}
              <div class="empty">
                <img src="/images/document-stack.svg" alt="" width="100" height="80" />
                <h2>Belum ada dokumen untuk pasien ini.</h2>
                <p>Gunakan tombol "Pilih laporan / lampiran" di atas untuk mengunggah berkas pemeriksaan ke profil pasien ini.</p>
              </div>
            {/each}
          {/if}
        </section>
      </main>
    {:else}
      <main class="patient-page">
        <section class="patient-card unavailable-panel" aria-labelledby="unavailable-records-title">
          <img src="/images/document-stack.svg" alt="" width="96" height="80" />
          <h2 id="unavailable-records-title">Layanan Rekam Medis Belum Tersedia di Backend</h2>
          <p>
            Endpoint backend untuk membaca, membuat, menyimpan draf/final, maupun melihat riwayat versi (version history) rekam medis belum diimplementasikan.
          </p>
          <p class="patient-note">
            Untuk membagikan hasil pemeriksaan atau laporan medis kepada <strong>{patient.name}</strong> saat ini, silakan gunakan tab <strong>Dokumen & Lampiran Pasien</strong> untuk mengunggah berkas laporan secara langsung.
          </p>
          <button class="patient-button" type="button" onclick={() => tab = 'documents'}>
            Buka Dokumen & Lampiran Pasien
          </button>
        </section>
      </main>
    {/if}
  </div>
{/if}

<dialog bind:this={previewDialog} onclose={() => detail = null} aria-labelledby="doctor-document-title">
  <form method="dialog"><button class="text-button" type="submit">Tutup</button></form>
  {#if detail}
    <h2 id="doctor-document-title">{detail.name}</h2>
    <p class="patient-note">{detail.source} · Ukuran asli {formatBytes(detail.size)} · ID Berkas #{detail.record.id}</p>
    <DocumentPreview
      file={detail}
      remoteFile={detail.record}
      onRemoteFileUpdated={handleRemoteUpdated}
      {onUnauthorized}
    />
  {/if}
</dialog>

<style>
  .eyebrow { margin: 0 0 12px; font-size: 11px; font-weight: 500; letter-spacing: .5px; }
  .overview { display: grid; grid-template-columns: 1fr 1fr 1.2fr; gap: 20px; margin-bottom: 32px; }
  .overview > div { padding: 26px 28px; }
  .overview span { font-size: 12px; }
  .overview strong { display: block; font-size: 32px; font-weight: 500; letter-spacing: -1.4px; margin-top: 10px; }
  .overview small { display: block; margin-top: 6px; font-size: 11px; color: #5b6470; }
  .overview-note { display: flex; align-items: center; justify-content: center; gap: 22px; background: #f4f8fd; }
  .overview-note p { font-size: 15px; line-height: 1.7; letter-spacing: -.3px; }
  h2 { margin: 0; font-size: 21px; font-weight: 550; letter-spacing: -.5px; }
  .list-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 24px; padding: 28px; }
  .list-heading p { margin: 8px 0 0; font-size: 12px; }
  .list-controls { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
  .list-controls input { width: 280px; max-width: 100%; height: 44px; padding: 0 14px; font-size: 12px; }
  .error-banner { margin: 0 28px 20px; padding: 16px; border: 1px solid #f0c9c5; border-radius: 12px; background: #fdf4f3; color: #8c2f25; display: flex; align-items: center; justify-content: space-between; gap: 16px; font-size: 12px; }
  .error-banner p { margin: 0; }
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
  dl { display: flex; flex-wrap: wrap; gap: 36px; margin: 0; }
  dt { font-size: 10px; margin-bottom: 8px; }
  dd { font-size: 13px; margin: 0; white-space: nowrap; }
  .doctor-tabs { display: flex; gap: 32px; border-bottom: 1px solid #d8dfe6; }
  .doctor-tabs button { padding: 16px 0; border: 0; border-bottom: 2px solid transparent; background: none; font-size: 13px; }
  .doctor-tabs button.active { color: #0069d4; border-bottom-color: #0071e3; }
  .document-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 24px; }
  .document-actions-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .file-picker { position: relative; cursor: pointer; }
  .file-picker input { position: absolute; inset: 0; opacity: 0; width: 100%; cursor: pointer; }
  .file-picker:focus-within { outline: 2px solid #0071e3; outline-offset: 4px; }
  .pending-upload-bar { padding: 20px 24px; margin-bottom: 20px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; background: #f4f8fd; border-color: #c9ddf8; font-size: 13px; }
  .pending-upload-actions { display: flex; gap: 10px; }
  .patient-message.error { border-color: #f0c9c5; background: #fdf4f3; color: #8c2f25; }
  .document-list { margin-top: 28px; overflow: hidden; }
  .document-row { width: 100%; display: flex; align-items: center; gap: 20px; border: 0; border-bottom: 1px solid #e1e6ec; padding: 24px 28px; text-align: left; background: #fff; }
  .document-row:last-child { border-bottom: 0; }
  .document-row:hover { background: #f6f9fd; }
  .document-row img { flex-shrink: 0; }
  .document-name { flex: 1; min-width: 0; }
  .document-name strong { font-size: 14px; font-weight: 550; overflow-wrap: anywhere; }
  .file-status { font-size: 10px; background: #f3f6fa; padding: 6px 10px; border: 1px solid #e0e6ee; border-radius: 7px; }
  .unavailable-panel { padding: 48px 32px; text-align: center; max-width: 680px; margin: 0 auto; }
  .unavailable-panel img { margin-bottom: 18px; }
  .unavailable-panel p { font-size: 13px; line-height: 1.8; margin: 14px auto 0; max-width: 55ch; }
  .unavailable-panel .patient-button { margin-top: 24px; }
  dialog { width: min(760px, calc(100% - 40px)); max-height: calc(100svh - 40px); padding: 28px; border: 1px solid #d1d8e0; border-radius: 22px; color: #111; }
  dialog::backdrop { background: #111b2b55; backdrop-filter: blur(5px); }
  dialog > form { display: flex; justify-content: flex-end; margin-bottom: 14px; }
  dialog h2 { overflow-wrap: anywhere; }
  @media (max-width: 900px) {
    .overview { grid-template-columns: 1fr 1fr; }
    .overview-note { display: none; }
    .summary-row { align-items: start; flex-direction: column; gap: 24px; }
  }
  @media (max-width: 680px) {
    .patient-summary, .doctor-tabs { width: calc(100% - 40px); }
    .overview { gap: 12px; grid-template-columns: 1fr; }
    .overview > div { padding: 20px; }
    .list-heading { align-items: stretch; flex-direction: column; padding: 24px; }
    .list-controls input { width: 100%; }
    th, td { padding-inline: 22px; }
    .document-row { padding: 20px; gap: 14px; flex-wrap: wrap; }
    .file-status { margin-left: 60px; }
    .document-name strong { font-size: 12px; }
    dl { gap: 20px; }
  }
</style>
