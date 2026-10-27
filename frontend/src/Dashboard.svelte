<script lang="ts">
  import { onMount, tick } from 'svelte'
  import DocumentPreview from './DocumentPreview.svelte'
  import type { DocumentCategory } from './patientExamples'
  import {
    ApiError,
    apiListFiles,
    apiUploadPatientFile,
    classifyFileCategory,
    classifyFileKind,
    formatBytes,
    type ApiFileRecord,
    type CurrentUser,
  } from './api'

  let {
    user,
    onOpenProfile,
    onUnauthorized,
  }: {
    user: CurrentUser
    onOpenProfile?: () => void
    onUnauthorized?: () => void
  } = $props()

  type DashboardItem = {
    name: string
    kind: string
    category: DocumentCategory
    size: number
    selected?: boolean
    record?: ApiFileRecord
  }

  const illustratedKinds = ['PDF', 'DOCX', 'PNG', 'MP4']
  const categories: DocumentCategory[] = ['Semua', 'Dokumen', 'Foto', 'Video']
  let category = $state<DocumentCategory>('Semua')
  let search = $state('')
  let selectedFile = $state<File | null>(null)
  let fileInput = $state<HTMLInputElement>()
  let dialog = $state<HTMLDialogElement>()
  let detail = $state<DashboardItem | null>(null)
  let notice = $state('')
  let noticeError = $state(false)
  let uploading = $state(false)
  let loadingFiles = $state(true)
  let loadError = $state('')
  let remoteFiles = $state<ApiFileRecord[]>([])

  const documents = $derived<DashboardItem[]>([
    ...(selectedFile
      ? [
          {
            name: selectedFile.name,
            kind: classifyFileKind(selectedFile.name),
            category: classifyFileCategory(selectedFile.name, selectedFile.type),
            size: selectedFile.size,
            selected: true,
          },
        ]
      : []),
    ...remoteFiles.map(record => ({
      name: record.original_name,
      kind: classifyFileKind(record.original_name),
      category: classifyFileCategory(record.original_name, record.original_type),
      size: record.original_size,
      record,
    })),
  ])

  const filtered = $derived(
    documents.filter(
      file =>
        (category === 'Semua' || file.category === category) &&
        file.name.toLowerCase().includes(search.toLowerCase().trim()),
    ),
  )
  const firstName = $derived(user.name.trim().split(/\s+/)[0] || user.username)

  onMount(() => {
    const controller = new AbortController()
    void fetchFiles(controller.signal)
    return () => controller.abort()
  })

  async function fetchFiles(signal?: AbortSignal) {
    loadingFiles = true
    loadError = ''
    try {
      const list = await apiListFiles(signal)
      remoteFiles = list
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          loadError = 'Akun ini tidak memiliki izin membaca daftar dokumen pasien.'
        } else if (err.isNetworkError) {
          loadError = 'Tidak dapat terhubung ke server untuk memuat daftar dokumen.'
        } else {
          loadError = 'Gagal memuat daftar dokumen dari server.'
        }
      } else {
        loadError = 'Gagal memuat daftar dokumen.'
      }
    } finally {
      loadingFiles = false
    }
  }

  function chooseFile(event: Event) {
    if (uploading) return
    const file = (event.currentTarget as HTMLInputElement).files?.[0]
    if (!file) return
    noticeError = false
    if (file.size > 8 * 1024 * 1024) {
      notice = 'Ukuran file maksimal 8 MiB.'
      noticeError = true
      if (fileInput) fileInput.value = ''
      return
    }
    selectedFile = file
    notice = 'Dokumen dipilih. Klik "Unggah dokumen" untuk menyimpan ke akun Anda (AES, DES, dan RC4).'
    category = 'Semua'
    search = ''
  }

  function clearFile() {
    if (uploading) return
    selectedFile = null
    notice = ''
    noticeError = false
    if (fileInput) fileInput.value = ''
  }

  async function submitUpload() {
    if (!selectedFile || uploading) return
    uploading = true
    notice = ''
    noticeError = false
    try {
      const saved = await apiUploadPatientFile(selectedFile)
      selectedFile = null
      if (fileInput) fileInput.value = ''
      notice = `Berkas "${saved.original_name}" berhasil disimpan ke server.`
      await fetchFiles()
    } catch (err) {
      noticeError = true
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          notice = 'Hanya akun pasien yang dapat mengunggah melalui halaman ini.'
        } else if (err.status === 404) {
          notice = 'Profil pasien belum dibuat. Lengkapi profil pasien terlebih dahulu sebelum mengunggah dokumen.'
        } else if (err.status === 413) {
          notice = 'Ukuran berkas melebihi batas 8 MiB.'
        } else if (err.status === 400) {
          notice = 'Berkas tidak valid atau tidak lengkap.'
        } else if (err.isNetworkError) {
          notice = 'Gagal terhubung ke server saat mengunggah berkas.'
        } else {
          notice = 'Gagal memproses dan menyimpan berkas di server.'
        }
      } else {
        notice = 'Terjadi kesalahan saat mengunggah berkas.'
      }
    } finally {
      uploading = false
    }
  }

  function handleRemoteUpdated(updated: ApiFileRecord) {
    remoteFiles = remoteFiles.map(item => (item.id === updated.id ? updated : item))
    if (detail?.record && detail.record.id === updated.id) {
      detail = { ...detail, record: updated }
    }
  }

  function describeSource(item: DashboardItem): string {
    if (item.selected) return 'Belum diunggah'
    if (!item.record) return 'Tersimpan'
    if (item.record.uploaded_by_user_id === user.id) return 'Diunggah oleh Anda'
    return 'Diunggah oleh dokter / rumah sakit'
  }

  async function showDetail(file: DashboardItem) {
    detail = file
    await tick()
    dialog?.showModal()
  }
</script>

<main class="dashboard">
  <header class="welcome">
    <div>
      <h1 tabindex="-1">Halo, {firstName}.</h1>
      <p class="welcome-copy">Lengkapi dan lihat dokumen untuk keperluan perawatan Anda.</p>
    </div>
  </header>

  <div class="dashboard-layout">
    <aside class="sidebar" aria-label="Profil dan upload">
      <section class="panel account-panel" aria-labelledby="profile-title">
        <div class="profile-heading">
          <img src="/images/portal-mark.svg" alt="" width="26" height="26" />
          <h2 id="profile-title">Profil Anda</h2>
        </div>
        <p class="profile-name">{user.name}</p>
        <p class="profile-username">@{user.username}</p>
        <div class="profile-footer"><span>Jenis akun</span><span class="profile-role">{user.role === 'patient' ? 'Pasien' : 'Dokter'}</span></div>
        {#if onOpenProfile}<button class="profile-link" type="button" onclick={onOpenProfile}>Lihat profil & KTP pasien</button>{/if}
      </section>

      <section class="panel upload-panel" aria-labelledby="upload-title">
        <img class="upload-art" src="/images/document-stack.svg" alt="" width="200" height="160" />
        <h2 id="upload-title">Tambah dokumen</h2>
        <p>Setiap berkas baru disimpan dengan representasi AES, DES, dan RC4 di server.</p>
        <input class="file-input" id="document-file" type="file" bind:this={fileInput} onchange={chooseFile} disabled={uploading} aria-label="Pilih dokumen" />
        <button class="choose-file" type="button" disabled={uploading} onclick={() => fileInput?.click()}>Pilih dokumen</button>
        <span class="file-limit">Maksimal 8 MiB per file</span>
        {#if selectedFile}
          <div class="selected-file">
            <span>{selectedFile.name} ({formatBytes(selectedFile.size)})</span>
            <div class="selected-actions">
              <button class="upload-submit" type="button" disabled={uploading} onclick={submitUpload}>
                {uploading ? 'Mengunggah…' : 'Unggah dokumen'}
              </button>
              <button type="button" disabled={uploading} onclick={clearFile}>Batalkan</button>
            </div>
          </div>
        {/if}
        <p class="upload-notice" class:error={noticeError} aria-live="polite">{notice}</p>
        {#if noticeError && notice.includes('Profil pasien belum dibuat') && onOpenProfile}
          <button class="profile-link" type="button" onclick={onOpenProfile}>Buka halaman Profil Pasien</button>
        {/if}
      </section>
    </aside>

    <section class="panel documents-panel" aria-labelledby="documents-title">
      <header class="documents-heading">
        <div>
          <h2 id="documents-title">Dokumen pribadi <span class="count">{documents.length}</span></h2>
          <p>Lampiran milik profil pasien Anda, termasuk laporan yang diunggah dokter.</p>
        </div>
        <button class="refresh-button" type="button" disabled={loadingFiles} onclick={() => fetchFiles()}>
          {loadingFiles ? 'Memuat…' : 'Muat ulang'}
        </button>
      </header>

      <div class="documents-toolbar">
        <div class="filters" aria-label="Jenis dokumen">
          {#each categories as item}
            <button type="button" class:active={category === item} aria-pressed={category === item} onclick={() => category = item}>{item}</button>
          {/each}
        </div>
        <div class="search-field">
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 5 5" /></svg>
          <input type="search" bind:value={search} aria-label="Cari dokumen" placeholder="Cari dokumen" />
        </div>
      </div>

      {#if loadError}
        <div class="load-error" role="alert">
          <p>{loadError}</p>
          <button type="button" onclick={() => fetchFiles()}>Coba lagi</button>
        </div>
      {/if}

      <div class="document-list">
        <table>
          <thead><tr><th scope="col">Nama dokumen</th><th scope="col" class="size-column">Ukuran</th><th scope="col" class="action-column">Detail</th></tr></thead>
          <tbody>
            {#if loadingFiles && !documents.length}
              <tr><td colspan="3" class="empty-state"><p>Memuat daftar dokumen dari server…</p></td></tr>
            {:else}
              {#each filtered as file (file.record ? `remote-${file.record.id}` : `local-${file.name}`)}
                <tr>
                  <td>
                    <div class="file-name">
                      <span class="file-icon" aria-hidden="true">
                        {#if illustratedKinds.includes(file.kind)}
                          <img class="file-illustration" src={`/images/file-${file.kind.toLowerCase()}.svg`} alt="" width="52" height="61" />
                        {:else}
                          <span class="file-symbol" class:video={file.category === 'Video'} class:photo={file.category === 'Foto'} aria-hidden="true">{file.kind}</span>
                        {/if}
                      </span>
                      <div>
                        <span class="document-name">{file.name}</span>
                        <span class="document-kind">
                          {describeSource(file)}
                          {#if file.record}
                            · Cipher: {(file.record.algorithms?.length ? file.record.algorithms : ['aes']).map(a => a.toUpperCase()).join('/')}
                          {/if}
                          <span class="mobile-size"> · {formatBytes(file.size)}</span>
                        </span>
                      </div>
                    </div>
                  </td>
                  <td class="size-column">{formatBytes(file.size)}</td>
                  <td class="action-column"><button class="detail-button" type="button" aria-label={`Lihat detail ${file.name}`} onclick={() => showDetail(file)}>Lihat</button></td>
                </tr>
              {:else}
                <tr>
                  <td colspan="3" class="empty-state">
                    {#if documents.length === 0}
                      <p>Belum ada dokumen yang tersimpan pada profil pasien Anda.</p>
                    {:else}
                      <p>Dokumen tidak ditemukan untuk filter ini.</p>
                      <button type="button" onclick={() => { search = ''; category = 'Semua' }}>Tampilkan semua</button>
                    {/if}
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
      <div class="documents-footer"><span>{filtered.length} dokumen ditampilkan</span></div>
    </section>
  </div>
</main>

<dialog bind:this={dialog} aria-labelledby="detail-title" onclose={() => detail = null}>
  {#if detail}
    <form method="dialog"><button class="close-dialog" aria-label="Tutup detail" type="submit">Tutup</button></form>
    {#if illustratedKinds.includes(detail.kind)}
      <img class="file-illustration detail-illustration" src={`/images/file-${detail.kind.toLowerCase()}.svg`} alt="" width="82" height="96" />
    {:else}
      <span class="file-symbol dialog-symbol" aria-hidden="true">{detail.kind}</span>
    {/if}
    <h2 id="detail-title">{detail.name}</h2>
    <p class="dialog-caption">{describeSource(detail)}</p>
    <dl>
      <div><dt>Jenis</dt><dd>{detail.category}</dd></div>
      <div><dt>Ukuran asli</dt><dd>{formatBytes(detail.size)}</dd></div>
      <div><dt>Status</dt><dd>{detail.selected ? 'Belum diunggah' : `ID #${detail.record?.id}`}</dd></div>
    </dl>
    <DocumentPreview
      file={detail}
      localFile={detail.selected ? selectedFile : null}
      remoteFile={detail.record ?? null}
      onRemoteFileUpdated={handleRemoteUpdated}
      {onUnauthorized}
    />
  {/if}
</dialog>

<style>
  .dashboard { display: block; width: calc(100% - 80px); max-width: 1280px; margin: 0 auto; padding: 58px 0 80px; }
  .welcome { margin-bottom: 38px; }
  h1 { margin: 0; font-size: clamp(32px, 4vw, 48px); font-weight: 550; line-height: 1.2; letter-spacing: -1.8px; overflow-wrap: anywhere; }
  .welcome-copy { margin: 14px 0 0; font-size: 15px; }
  .dashboard-layout { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 24px; align-items: start; }
  .sidebar { min-width: 0; display: grid; gap: 20px; }
  .panel { min-width: 0; background: #fff; border: 1px solid #d1d8e0; border-radius: 22px; }
  h2 { margin: 0; font-size: 18px; font-weight: 550; letter-spacing: -.5px; }
  dl { margin: 25px 0 0; }
  dl > div + div { margin-top: 20px; }
  dt { font-size: 11px; margin-bottom: 5px; }
  dd { margin: 0; font-size: 14px; font-weight: 500; overflow-wrap: anywhere; }
  .account-panel { padding: 28px; box-shadow: 0 2px 10px #15213a05; }
  .profile-heading { display: flex; align-items: center; gap: 10px; }
  .profile-heading img { object-fit: contain; }
  .profile-heading h2 { font-size: 12px; font-weight: 500; letter-spacing: 0; }
  .profile-name { margin: 28px 0 8px; font-size: 25px; font-weight: 550; letter-spacing: -.8px; line-height: 1.25; overflow-wrap: anywhere; text-wrap: balance; }
  .profile-username { margin: 0; font-size: 12px; overflow-wrap: anywhere; }
  .profile-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-top: 24px; margin-top: 28px; border-top: 1px solid #e1e6ec; font-size: 11px; }
  .profile-link { padding: 6px 0; margin-top: 18px; border: 0; background: none; color: #0071e3; font-size: 12px; }
  .profile-link:hover { text-decoration: underline; text-underline-offset: 3px; }
  .profile-role { padding: 5px 11px; border-radius: 7px; background: #f2f6fc; color: #0069d4; font-size: 11px; font-weight: 500; }
  .upload-panel { padding: 12px 22px 26px; text-align: center; }
  .upload-art { display: block; width: 180px; max-width: 100%; height: auto; margin: 2px auto 0; }
  .upload-panel h2 { max-width: 190px; margin: 0 auto; font-size: 21px; font-weight: 550; line-height: 1.3; letter-spacing: -.6px; text-wrap: balance; }
  .upload-panel > p { margin: 12px auto 22px; max-width: 210px; font-size: 12px; line-height: 1.7; text-wrap: pretty; }
  .file-input { display: none; }
  .choose-file { min-height: 44px; padding: 11px 24px; border: 0; border-radius: 100px; color: #fff; background: #0071e3; font-size: 12px; font-weight: 500; transition: background 200ms, transform 200ms; min-width: 136px; max-width: 100%; }
  .choose-file:hover:enabled { background: #0064cb; }
  .choose-file:active:enabled { transform: scale(.97); }
  .choose-file:disabled { opacity: .6; }
  .file-limit { display: block; font-size: 10px; margin-top: 13px; }
  .selected-file { display: grid; justify-items: center; gap: 10px; margin-top: 18px; font-size: 12px; }
  .selected-file > span { max-width: 100%; overflow-wrap: anywhere; }
  .selected-actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
  .upload-submit { min-height: 36px; padding: 8px 16px; border: 0; border-radius: 100px; background: #111; color: #fff; font-size: 11px; font-weight: 500; }
  .upload-submit:disabled { opacity: .6; }
  .selected-actions button:not(.upload-submit) { padding: 4px 8px; border: 0; background: none; color: #0071e3; font-size: 12px; }
  .upload-panel .upload-notice { margin: 12px auto 0; font-size: 11px; line-height: 1.6; }
  .upload-panel .upload-notice.error { color: #9f3a30; }
  .upload-notice:empty { display: none; }
  .documents-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 30px 30px 26px; }
  .documents-heading h2 { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; font-size: 20px; }
  .documents-heading p { margin: 9px 0 0; font-size: 12px; }
  .count { display: inline-grid; place-items: center; min-width: 25px; height: 25px; padding: 0 6px; border-radius: 8px; background: #f1f4f7; font-size: 11px; letter-spacing: 0; }
  .refresh-button { min-height: 36px; padding: 7px 14px; border: 1px solid #d1d8e0; border-radius: 10px; background: #fff; font-size: 11px; }
  .refresh-button:hover:enabled { background: #f4f7fb; }
  .documents-toolbar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 14px; padding: 0 30px 26px; }
  .filters { display: flex; padding: 4px; border: 1px solid #e0e5eb; border-radius: 11px; background: #f3f5f7; }
  .filters button { min-height: 34px; padding: 7px 12px; border: 0; border-radius: 8px; background: none; font-size: 11px; }
  .filters button.active { background: #fff; box-shadow: 0 0 0 1px #d8dfe7, 0 1px 4px #15213a0a; font-weight: 600; }
  .search-field { position: relative; flex: 1; min-width: 150px; max-width: 220px; }
  .search-field svg { position: absolute; top: 13px; left: 13px; pointer-events: none; }
  .search-field input { height: 42px; padding: 0 12px 0 38px; border-radius: 10px; border-color: #d1d8e0; font-size: 12px; }
  .search-field input:focus { border-color: #0071e3; }
  .search-field input::placeholder { font-size: 12px; }
  .load-error { margin: 0 30px 20px; padding: 14px 16px; border: 1px solid #f0c9c5; border-radius: 12px; background: #fdf4f3; color: #8c2f25; font-size: 12px; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .load-error p { margin: 0; }
  .load-error button { border: 1px solid #d9a39d; background: #fff; border-radius: 8px; padding: 6px 12px; font-size: 11px; color: #8c2f25; }
  table { border-collapse: collapse; table-layout: fixed; width: 100%; text-align: left; }
  th { padding: 13px 12px; border-block: 1px solid #e1e6ec; background: #fafbfc; font-size: 10px; font-weight: 500; }
  th:first-child { width: 65%; padding-left: 30px; }
  td { padding: 23px 12px; font-size: 12px; vertical-align: middle; border-bottom: 1px solid #e1e6ec; }
  td:first-child { padding-left: 30px; }
  tbody tr:last-child td { border-bottom: 0; }
  .size-column { width: 19%; white-space: nowrap; }
  .action-column { width: 16%; padding-right: 30px; text-align: right; }
  .file-name { display: flex; align-items: center; gap: 14px; min-width: 0; }
  .file-name > div { min-width: 0; }
  .file-icon { display: grid; place-items: center; flex-shrink: 0; width: 52px; height: 61px; }
  .file-illustration { display: block; flex-shrink: 0; width: 52px; height: 61px; object-fit: contain; }
  .file-symbol { flex-shrink: 0; display: inline-grid; place-items: center; width: 43px; height: 49px; border: 1px solid #d9e4f6; border-radius: 9px; background: #f1f6ff; color: #0069d4; font-size: 9px; font-weight: 600; }
  .file-symbol.photo { border-color: #e0e0ed; background: #f6f4fb; color: #534677; }
  .file-symbol.video { border-color: #dde6e6; background: #f0f7f7; color: #21635d; }
  .document-name { display: block; font-weight: 500; font-size: 13px; overflow-wrap: anywhere; }
  .document-kind { display: block; margin-top: 5px; font-size: 10px; }
  .mobile-size { display: none; }
  .detail-button { min-height: 36px; padding: 7px 12px; border-radius: 9px; border: 1px solid #d1d8e0; background: #fff; font-size: 11px; }
  .detail-button:hover { background: #f4f7fb; border-color: #bacce5; }
  .documents-footer { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 12px; padding: 20px 30px; border-top: 1px solid #e1e6ec; font-size: 10px; }
  .empty-state { text-align: center; padding: 56px 20px; }
  .empty-state p { margin: 0 0 12px; font-size: 13px; }
  .empty-state button { border: 0; padding: 6px; background: none; color: #0071e3; font-size: 12px; }
  dialog { width: calc(100% - 40px); max-width: 760px; max-height: calc(100svh - 40px); padding: 28px; border: 1px solid #d1d8e0; border-radius: 22px; color: #111; background: #fff; box-shadow: 0 24px 100px #101e3826; }
  dialog::backdrop { background: #111b3038; backdrop-filter: blur(5px); }
  dialog form { display: flex; justify-content: flex-end; margin-bottom: 16px; }
  .close-dialog { min-height: 36px; padding: 7px 12px; border: 1px solid #d1d8e0; border-radius: 20px; background: #fff; font-size: 11px; }
  .dialog-symbol { width: 48px; height: 56px; margin-bottom: 18px; }
  .detail-illustration { width: 82px; height: 96px; margin-bottom: 18px; }
  dialog h2 { font-size: 24px; overflow-wrap: anywhere; }
  .dialog-caption { font-size: 12px; margin: 12px 0 0; }
  dialog dl { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px; padding-top: 20px; border-top: 1px solid #d1d8e0; }
  dialog dl > div { min-width: 0; }
  dialog dl > div + div { margin-top: 0; }
  dialog dt { margin: 0; }
  dialog dd { margin-top: 8px; text-align: left; font-size: 12px; }
  @media (max-width: 1000px) {
    .dashboard-layout { grid-template-columns: 240px minmax(0, 1fr); gap: 20px; }
    .account-panel { padding: 24px; }
    .upload-panel { padding-inline: 16px; }
    .documents-heading { padding: 26px 22px; }
    .documents-toolbar { padding-inline: 22px; }
    td:first-child, th:first-child { padding-left: 22px; }
    .action-column { padding-right: 22px; }
    .search-field { max-width: none; }
  }
  @media (max-width: 760px) {
    .dashboard { width: calc(100% - 40px); padding-top: 38px; }
    .welcome { margin-bottom: 28px; }
    .welcome-copy { font-size: 13px; }
    .dashboard-layout { grid-template-columns: minmax(0, 1fr); }
    .sidebar { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; gap: 24px; }
    .upload-art { width: 150px; }
  }
  @media (max-width: 480px) {
    .sidebar { grid-template-columns: minmax(0, 1fr); }
    .account-panel { padding: 26px; }
    .profile-name { margin-top: 22px; }
    .profile-footer { padding-top: 20px; margin-top: 24px; }
    .upload-panel { padding: 12px 26px 24px; }
    .documents-heading { padding: 24px 18px; }
    .documents-heading h2 { font-size: 18px; }
    .documents-heading p { font-size: 11px; }
    .documents-toolbar { padding: 0 18px 22px; }
    .filters { width: 100%; }
    .filters button { flex: 1; padding-inline: 8px; }
    .search-field { width: 100%; flex-basis: 100%; }
    .size-column { display: none; }
    th:first-child { width: 75%; }
    td:first-child, th:first-child { padding-left: 18px; }
    td { padding-block: 20px; }
    .action-column { width: 25%; padding-right: 18px; }
    .file-name { gap: 10px; }
    .file-symbol { width: 35px; height: 43px; font-size: 8px; }
    .file-icon { width: 42px; height: 49px; }
    .file-name .file-illustration { width: 42px; height: 49px; }
    .document-name { font-size: 12px; }
    .mobile-size { display: inline; }
    .documents-footer { padding-inline: 18px; font-size: 9px; }
  }
</style>
