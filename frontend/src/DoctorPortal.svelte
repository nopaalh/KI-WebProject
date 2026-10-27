<script lang="ts">
  import { tick } from 'svelte'
  import MedicalRecords from './MedicalRecords.svelte'
  import DocumentPreview from './DocumentPreview.svelte'
  import { exampleDoctorPatients, type DoctorDocument } from './doctorExamples'
  import type { MedicalRecord, MedicalVersion } from './patientExamples'

  let { user }: { user: { name: string; username: string } } = $props()
  let patients = $state(structuredClone(exampleDoctorPatients))
  let selectedId = $state<number | null>(null)
  let search = $state('')
  let tab = $state<'records' | 'documents'>('records')
  let recordId = $state<number | null>(null)
  let editing = $state(false)
  let editId = $state<number | null>(null)
  let title = $state('')
  let department = $state('')
  let summary = $state('')
  let note = $state('')
  let original = ''
  let notice = $state('')
  let fileError = $state('')
  let detail = $state<DoctorDocument | null>(null)
  let previewDialog = $state<HTMLDialogElement>()
  let editorForm = $state<HTMLFormElement>()
  const patient = $derived(patients.find(item => item.id === selectedId))
  const filtered = $derived(patients.filter(item => `${item.name} ${item.reference}`.toLowerCase().includes(search.trim().toLowerCase())))
  const draftCount = $derived(patients.reduce((count, item) => count + item.records.filter(record => record.versions[0]?.status === 'Draf').length, 0))

  async function openPatient(id: number) {
    selectedId = id
    recordId = null
    tab = 'records'
    notice = ''
    fileError = ''
    await tick()
    document.querySelector<HTMLElement>('.patient-summary h1')?.focus()
  }

  async function edit(record?: MedicalRecord) {
    editId = record?.id ?? null
    title = record?.title ?? ''
    department = record?.department ?? ''
    summary = record?.versions[0]?.summary ?? ''
    note = record?.versions[0]?.note ?? ''
    original = JSON.stringify([title, department, summary, note])
    editing = true
    notice = ''
    await tick()
    editorForm?.querySelector('input')?.focus()
  }

  function cancelEdit() {
    if (original !== JSON.stringify([title, department, summary, note]) && !window.confirm('Buang perubahan catatan yang belum disimpan?')) return
    editing = false
  }

  function save(status: MedicalVersion['status']) {
    if (!patient || !editorForm?.reportValidity()) return
    if (![title, department, summary, note].every(value => value.trim())) {
      notice = 'Lengkapi semua kolom dengan isi yang benar.'
      return
    }
    if (status === 'Final' && !window.confirm('Tandai catatan sebagai final? Revisi berikutnya akan menjadi versi baru.')) return
    const record = patient.records.find(item => item.id === editId)
    const version: MedicalVersion = {
      version: (record?.versions[0]?.version ?? 0) + 1,
      date: new Intl.DateTimeFormat('id-ID', { dateStyle: 'long', timeStyle: 'short' }).format(new Date()),
      author: user.name,
      status,
      summary: summary.trim(),
      note: note.trim(),
    }
    if (record) {
      record.versions.unshift(version)
      recordId = record.id
    } else {
      const id = Math.max(0, ...patient.records.map(item => item.id)) + 1
      patient.records.unshift({ id, title: title.trim(), department: department.trim(), reference: `${patient.reference}-RM-${String(id).padStart(3, '0')}`, versions: [version] })
      recordId = id
    }
    editing = false
    notice = `Versi ${version.version} disimpan sebagai ${status.toLowerCase()} di halaman ini. Belum dikirim ke rumah sakit; perubahan hilang saat halaman dimuat ulang.`
  }

  function selectFiles(event: Event) {
    const input = event.currentTarget as HTMLInputElement
    const selected = Array.from(input.files ?? [])
    if (!patient || !selected.length) return
    fileError = ''
    if (selected.some(file => file.size > 8 * 1024 * 1024 || !/\.(pdf|docx|png|jpe?g|mp4|webm)$/i.test(file.name))) {
      fileError = 'Pilih PDF, DOCX, PNG, JPG, MP4, atau WebM dengan ukuran maksimal 8 MB per file.'
      input.value = ''
      return
    }
    patient.documents.push(...selected.map(file => ({
      name: file.name, size: file.size, kind: file.name.split('.').pop()!.toUpperCase(),
      category: /\.(png|jpe?g)$/i.test(file.name) ? 'Foto' as const : /\.(mp4|webm)$/i.test(file.name) ? 'Video' as const : 'Dokumen' as const,
      localFile: file, selected: true, source: 'Rumah sakit' as const,
    })))
    notice = `${selected.length} berkas dipilih untuk ${patient.name}. Belum diunggah; pilihan hilang saat halaman dimuat ulang.`
    input.value = ''
  }

  async function openDocument(file: DoctorDocument) {
    detail = file
    await tick()
    previewDialog?.showModal()
  }

  function fileIcon(file: DoctorDocument) {
    return file.category === 'Foto' ? 'png' : file.category === 'Video' ? 'mp4' : file.kind === 'PDF' ? 'pdf' : 'docx'
  }
</script>

<svelte:head><title>{patient ? `${patient.name} · Ruang dokter` : 'Pasien · Ruang dokter'}</title></svelte:head>

{#if !patient}
  <main class="patient-page">
    <header class="patient-heading"><p class="eyebrow">Ruang dokter</p><h1 class="patient-title">Pasien Anda.</h1><p class="patient-subtitle">Buka profil pasien untuk melanjutkan pencatatan pemeriksaan.</p></header>
    <div class="overview"><div class="patient-card"><span>Pasien terdaftar</span><strong>{patients.length.toString().padStart(2, '0')}</strong></div><div class="patient-card"><span>Catatan dalam penyusunan</span><strong>{draftCount.toString().padStart(2, '0')}</strong></div><div class="patient-card overview-note"><img src="/images/document-stack.svg" alt="" width="80" height="70" /><p>Setiap pemeriksaan,<br />tercatat dengan jelas.</p></div></div>
    <section class="patient-card patient-list" aria-labelledby="patients-title">
      <div class="list-heading"><div><h2 id="patients-title">Daftar pasien</h2><p>Data contoh untuk mencoba alur layanan.</p></div><input type="search" placeholder="Cari nama atau nomor pasien" aria-label="Cari pasien" bind:value={search} /></div>
      <div class="table-scroll"><table><thead><tr><th>Pasien</th><th>Nomor pasien</th><th>Rekam medis</th><th><span class="action-label">Tindakan</span></th></tr></thead><tbody>{#each filtered as item}<tr><td><div class="patient-name"><span class="avatar" aria-hidden="true">{item.name.split(' ').map(part => part[0]).join('')}</span><span><strong>{item.name}</strong><small>{item.age} tahun</small></span></div></td><td>{item.reference}</td><td>{item.records.length} pemeriksaan</td><td><button class="patient-button secondary" type="button" aria-label={`Buka pasien ${item.name}`} onclick={() => openPatient(item.id)}>Buka pasien</button></td></tr>{:else}<tr><td colspan="4" class="empty"><h3>Pasien tidak ditemukan.</h3><p>Coba nama atau nomor pasien lain.</p><button class="text-button" type="button" onclick={() => search = ''}>Tampilkan semua pasien</button></td></tr>{/each}</tbody></table></div>
    </section>
    <p class="patient-note">Seluruh profil, dokumen, dan catatan pada halaman ini merupakan data contoh.</p>
  </main>
{:else}
  <div class="doctor-workspace">
    <section class="patient-summary" aria-label="Pasien yang sedang dibuka">
      <button class="text-button" type="button" disabled={editing} onclick={() => selectedId = null}>Semua pasien</button>
      <div class="summary-row"><div class="summary-name"><span class="avatar large" aria-hidden="true">{patient.name.split(' ').map(part => part[0]).join('')}</span><div><p class="eyebrow">{patient.reference} · Data contoh</p><h1 tabindex="-1">{patient.name}</h1></div></div><dl><div><dt>Usia</dt><dd>{patient.age} tahun</dd></div><div><dt>Berat badan</dt><dd>{patient.weight} kg</dd></div><div><dt>Tinggi badan</dt><dd>{patient.height} cm</dd></div></dl></div>
    </section>
    <nav class="doctor-tabs" aria-label="Data pasien">{#each [{ id: 'records' as const, label: 'Rekam medis' }, { id: 'documents' as const, label: 'Dokumen pasien' }] as item}<button type="button" disabled={editing} aria-current={tab === item.id ? 'page' : undefined} class:active={tab === item.id} onclick={() => { tab = item.id; notice = ''; fileError = '' }}>{item.label}</button>{/each}</nav>
    {#if editing}
      <main class="patient-page editor-page">
        <header class="patient-heading"><h1 class="patient-title">{editId ? 'Revisi catatan' : 'Catatan baru'}</h1><p class="patient-subtitle">{patient.name} · {editId ? 'Perubahan akan disimpan sebagai versi baru.' : 'Mulai catatan pemeriksaan untuk pasien ini.'}</p></header>
        <form class="patient-card record-editor" bind:this={editorForm} onsubmit={event => { event.preventDefault(); save('Draf') }}>
          <div class="editor-fields"><label>Nama pemeriksaan<input bind:value={title} required maxlength="120" readonly={editId !== null} placeholder="Contoh: Pemeriksaan MRI" /></label><label>Unit layanan<input bind:value={department} required maxlength="80" readonly={editId !== null} placeholder="Contoh: Radiologi" /></label></div>
          <label>Ringkasan<input bind:value={summary} required maxlength="240" placeholder="Tuliskan perkembangan pemeriksaan" /></label>
          <label>Catatan pemeriksaan<textarea bind:value={note} required maxlength="20000" rows="10" placeholder="Tuliskan hasil, perkembangan, atau catatan pemeriksaan pasien."></textarea></label>
          <p class="patient-note">Draf dapat dibaca pasien ketika layanan rekam medis tersambung. Pastikan setiap catatan sudah layak dibagikan. Pada tampilan ini, perubahan hanya disimpan sementara.</p>
          {#if notice}<p class="patient-message" role="status">{notice}</p>{/if}
          <div class="editor-actions"><button class="text-button cancel-edit" type="button" onclick={cancelEdit}>Batal</button><button class="patient-button secondary" type="submit">Simpan draf</button><button class="patient-button" type="button" onclick={() => save('Final')}>Tandai final</button></div>
        </form>
      </main>
    {:else if tab === 'records'}
      <div class="record-actions"><p class="patient-note" role="status">{notice || 'Data contoh. Catatan belum terhubung ke layanan rumah sakit.'}</p><button class="patient-button" type="button" onclick={() => edit()}>Catatan baru</button></div>
      {#key patient.id}<MedicalRecords records={patient.records} bind:selectedId={recordId} onEdit={edit} />{/key}
    {:else}
      <main class="patient-page">
        <header class="patient-heading document-heading"><div><h1 class="patient-title">Dokumen pasien</h1><p class="patient-subtitle">Berkas dari pasien dan dokumen pendukung pemeriksaan.</p></div><label class="patient-button file-picker">Tambah dokumen<input type="file" multiple accept=".pdf,.docx,.png,.jpg,.jpeg,.mp4,.webm" onchange={selectFiles} /></label></header>
        {#if fileError}<p class="patient-message" role="alert">{fileError}</p>{/if}
        {#if notice}<p class="patient-message" role="status">{notice}</p>{/if}
        <section class="patient-card document-list" aria-label="Daftar dokumen pasien">{#each patient.documents as file}<button class="document-row" type="button" onclick={() => openDocument(file)}><img src={`/images/file-${fileIcon(file)}.svg`} alt="" width="46" height="54" /><span class="document-name"><strong>{file.name}</strong><small>{file.kind} · {(file.size / 1024 / 1024).toLocaleString('id-ID', { maximumFractionDigits: 2 })} MB · {file.source}</small></span><span class="file-status">{file.localFile ? 'Belum diunggah' : 'Contoh dokumen'}</span></button>{:else}<div class="empty"><img src="/images/document-stack.svg" alt="" width="100" height="80" /><h2>Belum ada dokumen.</h2><p>Tambahkan dokumen pendukung untuk pasien ini.</p></div>{/each}</section>
        <p class="patient-note">File yang dipilih hanya tersedia di halaman ini. Belum diunggah atau dibagikan ke akun pasien.</p>
      </main>
    {/if}
  </div>
{/if}

<dialog bind:this={previewDialog} onclose={() => detail = null} aria-labelledby="doctor-document-title"><form method="dialog"><button class="text-button" type="submit">Tutup</button></form>{#if detail}<h2 id="doctor-document-title">{detail.name}</h2><p class="patient-note">{detail.source} · {detail.localFile ? 'File lokal, belum diunggah' : 'Contoh dokumen'}</p><DocumentPreview file={detail} localFile={detail.localFile} />{/if}</dialog>

<style>
  .eyebrow { margin: 0 0 12px; font-size: 11px; font-weight: 500; letter-spacing: .5px; }
  .overview { display: grid; grid-template-columns: 1fr 1fr 1.2fr; gap: 20px; margin-bottom: 32px; }
  .overview > div { padding: 26px 28px; }
  .overview span { font-size: 12px; }
  .overview strong { display: block; font-size: 42px; font-weight: 500; letter-spacing: -2px; margin-top: 10px; }
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
  .avatar { display: grid; place-items: center; flex-shrink: 0; width: 42px; height: 42px; border: 1px solid #d6e5f7; border-radius: 14px; background: #edf5ff; color: #0069d4; font-size: 12px; }
  .avatar.large { width: 60px; height: 60px; border-radius: 20px; font-size: 18px; }
  .empty, td.empty { padding: 48px 24px; text-align: center; white-space: normal; }
  .empty h3 { font-size: 17px; font-weight: 550; }
  .empty p { font-size: 12px; }
  .empty img { margin-bottom: 20px; }
  .patient-summary, .doctor-tabs, .record-actions { width: calc(100% - 80px); max-width: 1280px; margin: 0 auto; }
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
  .record-actions { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding-top: 30px; }
  .record-actions .patient-note { margin: 0; max-width: 75ch; }
  .record-actions > button { flex-shrink: 0; }
  .record-editor { max-width: 900px; padding: 32px; }
  .record-editor label { display: grid; gap: 10px; margin-bottom: 24px; font-size: 12px; }
  .editor-fields { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; }
  .record-editor input { height: 48px; padding: 0 14px; font-size: 13px; }
  .record-editor input[readonly] { background: #f6f8fa; }
  textarea { width: 100%; resize: vertical; min-height: 180px; border: 1px solid #cbd1d8; border-radius: 12px; background: #fff; padding: 14px; font: inherit; font-size: 14px; line-height: 1.8; color: inherit; }
  textarea:focus { outline: 2px solid #b9d9ff; border-color: #0071e3; }
  .editor-actions { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 28px; padding-top: 24px; border-top: 1px solid #e1e6ec; }
  .cancel-edit { margin-right: auto; padding-inline: 10px; }
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
  @media (max-width: 680px) { .patient-summary, .doctor-tabs, .record-actions { width: calc(100% - 40px); } .overview { gap: 12px; } .overview > div { padding: 20px; } .overview strong { font-size: 34px; } .list-heading { align-items: stretch; flex-direction: column; padding: 24px; } .list-heading input { width: 100%; } th, td { padding-inline: 22px; } .record-editor { padding: 24px; } .editor-fields { grid-template-columns: 1fr; gap: 0; } .record-actions { align-items: start; flex-direction: column-reverse; gap: 14px; } .document-row { padding: 20px; gap: 14px; flex-wrap: wrap; } .file-status { margin-left: 60px; } .document-name strong { font-size: 12px; } dl { gap: 26px; } .editor-actions .cancel-edit { width: 100%; margin: 0; text-align: left; } }
</style>
