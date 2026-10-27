<script lang="ts">
  import type { MedicalRecord } from './patientExamples'

  let { records, selectedId = $bindable<number | null>(null), onEdit }: { records: MedicalRecord[]; selectedId?: number | null; onEdit?: (record: MedicalRecord) => void } = $props()
  let filter = $state('Semua')
  let search = $state('')
  let revision = $state<number | null>(null)
  const record = $derived(records.find(item => item.id === selectedId))
  const version = $derived(record?.versions.find(item => item.version === revision) ?? record?.versions[0])
  const filtered = $derived(records.filter(item =>
    (filter === 'Semua' || item.versions[0]?.status === filter) &&
    `${item.title} ${item.department}`.toLowerCase().includes(search.toLowerCase().trim()),
  ))

  function openRecord(id: number) {
    revision = null
    selectedId = id
  }
</script>

<main class="patient-page">
  {#if record && version}
    <button class="text-button back-to-records" type="button" onclick={() => selectedId = null}>Kembali ke rekam medis</button>
    <header class="patient-heading"><div class="record-title"><h1 class="patient-title" tabindex="-1">{record.title}</h1><span class="record-status" class:draft={version.status === 'Draf'}>{version.status}</span></div><p class="patient-subtitle">{record.department} · {record.reference}</p></header>
    <div class="record-layout">
      <section class="patient-card medical-note" aria-labelledby="note-title">
        <div class="note-header"><h2 id="note-title">Catatan pemeriksaan</h2>{#if onEdit && version.version === record.versions[0]?.version}<button class="patient-button secondary" type="button" onclick={() => onEdit?.(record)}>Revisi catatan</button>{:else}<span class="example-label">Data contoh</span>{/if}</div>
        {#if version.version !== record.versions[0]?.version}<div class="version-banner" role="status">Anda sedang membaca versi terdahulu. <button class="text-button" type="button" onclick={() => revision = null}>Lihat versi terbaru</button></div>{/if}
        <dl class="record-metadata"><div><dt>Ditulis oleh</dt><dd>{version.author}</dd></div><div><dt>Terakhir diperbarui</dt><dd>{version.date}</dd></div></dl>
        <h3>{version.summary}</h3><p class="note-content">{version.note}</p>
        <div class="record-explanation">{version.status === 'Draf' ? 'Catatan masih disusun oleh dokter dan dapat berubah. Anda dapat melihat pembaruannya di halaman ini.' : 'Catatan ini sudah ditandai final oleh dokter.'}</div>
        <p class="patient-note">{onEdit ? 'Data contoh. Revisi pada tampilan ini hanya disimpan sementara dan belum dikirim ke rumah sakit.' : 'Rekam medis hanya dapat diperbarui oleh pihak rumah sakit. Data yang ditampilkan di sini adalah contoh, bukan hasil pemeriksaan Anda.'}</p>
      </section>
      <aside class="patient-card history-panel" aria-labelledby="history-title">
        <h2 id="history-title">Riwayat perubahan</h2><p>Setiap catatan memiliki penulis dan waktu pembaruan.</p>
        <ol class="history-list">
          {#each record.versions as item}
            <li><button type="button" class:active={item.version === version.version} aria-pressed={item.version === version.version} onclick={() => revision = item.version}><span class="version-name">Versi {item.version}{#if item.version === record.versions[0]?.version}<span>Terbaru</span>{/if}</span><span class="version-date">{item.date}</span><span class="version-author">{item.author}</span><span class="version-summary">{item.summary}</span></button></li>
          {/each}
        </ol>
      </aside>
    </div>
  {:else}
    <header class="patient-heading"><h1 class="patient-title" tabindex="-1">Rekam medis</h1><p class="patient-subtitle">{onEdit ? 'Kelola catatan pemeriksaan pasien dan tinjau setiap revisinya.' : 'Ikuti perkembangan pemeriksaan dan baca catatan dari dokter.'}</p></header>
    <section class="patient-card records-panel" aria-labelledby="records-title">
      <div class="records-heading"><div><h2 id="records-title">Riwayat pemeriksaan</h2><p>Catatan contoh untuk memperlihatkan alur pemeriksaan (layanan rekam medis backend belum tersedia).</p></div><span class="example-label">Data contoh</span></div>
      <div class="records-toolbar"><div class="record-filters" aria-label="Status rekam medis">{#each ['Semua', 'Draf', 'Final'] as item}<button type="button" aria-pressed={filter === item} class:active={filter === item} onclick={() => filter = item}>{item}</button>{/each}</div><input class="record-search" type="search" aria-label="Cari pemeriksaan" placeholder="Cari pemeriksaan" bind:value={search} /></div>
      <ul class="record-list">
        {#each filtered as item}
          <li><button type="button" class="record-row" aria-label={`Baca ${item.title}`} onclick={() => openRecord(item.id)}><img src="/images/document-stack.svg" alt="" width="76" height="61" /><span class="record-description"><strong>{item.title}</strong><span>{item.department} · {item.versions[0]?.author}</span><span>{item.versions[0]?.date}</span></span><span class="record-status" class:draft={item.versions[0]?.status === 'Draf'}>{item.versions[0]?.status}</span></button></li>
        {:else}
          <li class="records-empty"><h3>Pemeriksaan tidak ditemukan.</h3><p>Coba kata kunci atau status lain.</p><button class="text-button" type="button" onclick={() => { search = ''; filter = 'Semua' }}>Tampilkan semua pemeriksaan</button></li>
        {/each}
      </ul>
      <p class="records-footer">{filtered.length} pemeriksaan · Catatan hanya dapat dibaca oleh pasien</p>
    </section>
  {/if}
</main>

<style>
  .back-to-records { margin-bottom: 24px; }
  .record-title { display: flex; flex-wrap: wrap; align-items: center; gap: 20px; }
  .record-status { flex-shrink: 0; display: inline-block; padding: 6px 12px; border: 1px solid #d8dfe6; border-radius: 8px; background: #f5f7f8; font-size: 11px; font-weight: 500; }
  .record-status.draft { background: #eef5ff; color: #0069d4; border-color: #d3e4fa; }
  .record-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 24px; align-items: start; }
  .medical-note, .history-panel { padding: 30px; }
  .note-header, .records-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 16px; }
  h2 { margin: 0; font-size: 20px; font-weight: 550; letter-spacing: -.5px; }
  .example-label { font-size: 10px; padding: 6px 10px; border-radius: 6px; background: #f2f4f7; }
  .record-metadata { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; margin: 30px 0; padding-bottom: 28px; border-bottom: 1px solid #e1e6ec; }
  dt { font-size: 11px; margin-bottom: 8px; }
  dd { margin: 0; font-size: 13px; overflow-wrap: anywhere; }
  h3 { margin: 0; font-size: 20px; line-height: 1.5; font-weight: 550; letter-spacing: -.4px; }
  .note-content { font-size: 14px; line-height: 1.9; max-width: 65ch; margin: 18px 0 28px; }
  .record-explanation { background: #f4f7fb; border-radius: 12px; padding: 18px; font-size: 12px; line-height: 1.8; }
  .version-banner { margin-top: 24px; padding: 14px; border: 1px solid #d4e4fa; border-radius: 10px; background: #f2f7ff; font-size: 12px; line-height: 1.8; }
  .version-banner button { margin-left: 4px; }
  .history-panel > p { font-size: 12px; line-height: 1.8; margin: 12px 0 24px; }
  .history-list { margin: 0; padding: 0; list-style: none; }
  .history-list li + li { margin-top: 12px; }
  .history-list button { display: block; width: 100%; border: 1px solid #dce2e9; border-radius: 12px; padding: 16px; text-align: left; background: #fff; transition: border-color 180ms, background 180ms; }
  .history-list button:hover { border-color: #9bbce5; }
  .history-list button.active { background: #f3f7ff; border-color: #9bbce5; }
  .version-name { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px; font-size: 13px; font-weight: 600; }
  .version-name > span { font-size: 10px; font-weight: 400; color: #0071e3; }
  .version-date, .version-author, .version-summary { display: block; margin-top: 8px; font-size: 11px; line-height: 1.7; }
  .version-summary { margin-top: 12px; }
  .records-heading { padding: 30px; }
  .records-heading p { font-size: 12px; margin: 10px 0 0; }
  .records-toolbar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; padding: 0 30px 26px; }
  .record-filters { display: flex; padding: 4px; background: #f3f5f7; border: 1px solid #e0e5eb; border-radius: 11px; }
  .record-filters button { min-height: 36px; border: 0; border-radius: 8px; padding: 8px 18px; font-size: 12px; background: transparent; }
  .record-filters button.active { background: #fff; box-shadow: 0 0 0 1px #d8dfe7; }
  .record-search { width: 240px; max-width: 100%; height: 44px; font-size: 13px; padding: 0 14px; }
  .record-list { list-style: none; padding: 0; margin: 0; }
  .record-list li { border-top: 1px solid #e1e6ec; }
  .record-row { width: 100%; display: flex; align-items: center; gap: 18px; padding: 26px 30px; border: 0; background: #fff; text-align: left; transition: background 180ms; }
  .record-row:hover { background: #f6f9fd; }
  .record-row img { object-fit: contain; flex-shrink: 0; }
  .record-description { flex: 1; min-width: 0; }
  .record-description strong { display: block; font-size: 16px; font-weight: 550; overflow-wrap: anywhere; }
  .record-description > span { display: block; font-size: 12px; line-height: 1.7; margin-top: 7px; }
  .records-footer { padding: 20px 30px; border-top: 1px solid #e1e6ec; margin: 0; font-size: 11px; }
  .records-empty { text-align: center; padding: 50px 24px; }
  .records-empty h3 { font-size: 18px; }
  .records-empty p { font-size: 13px; }
  @media (max-width: 900px) { .record-layout { grid-template-columns: minmax(0, 1fr); } .history-list { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; } .history-list li + li { margin-top: 0; } .history-list button { height: 100%; } }
  @media (max-width: 540px) { .medical-note, .history-panel, .records-heading { padding: 24px; } .record-metadata { grid-template-columns: minmax(0, 1fr); } .history-list { grid-template-columns: minmax(0, 1fr); } .records-toolbar { padding: 0 24px 24px; } .record-search { width: 100%; } .record-row { padding: 22px; gap: 10px; flex-wrap: wrap; } .record-row img { width: 48px; height: 48px; } .record-description strong { font-size: 14px; } .record-row .record-status { margin-left: 58px; } .records-footer { padding-inline: 24px; line-height: 1.8; } }
</style>
