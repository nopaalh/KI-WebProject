<script lang="ts">
  let { file, localFile = null }: { file: { name: string; kind: string; category: string }; localFile?: File | null } = $props()
  let source = $state('')
  let loading = $state(true)
  let error = $state('')
  const media = $derived(file.kind === 'PDF' ? 'pdf' : file.category === 'Foto' ? 'image' : file.category === 'Video' ? 'video' : 'other')

  $effect(() => {
    const selected = localFile
    const kind = media
    let active = true
    let url = ''
    source = ''
    error = ''
    loading = kind !== 'other'
    if (!selected) {
      source = kind === 'pdf' ? '/examples/document-example.pdf' : kind === 'image' ? '/examples/identity-example.png' : kind === 'video' ? '/examples/video-example.webm' : ''
    } else {
      void (async () => {
        try {
          if (kind === 'pdf' && await selected.slice(0, 5).text() !== '%PDF-') throw new Error()
          if (!active) return
          url = URL.createObjectURL(kind === 'pdf' ? new Blob([selected], { type: 'application/pdf' }) : selected)
          source = url
        } catch {
          if (active) { error = 'File ini tidak dapat ditampilkan. Pilih file lain.'; loading = false }
        }
      })()
    }
    return () => { active = false; if (url) URL.revokeObjectURL(url) }
  })

  function failed() {
    loading = false
    error = 'File ini tidak dapat ditampilkan oleh browser. Anda dapat membuka salinan lokalnya di perangkat.'
  }
</script>

<section class="preview" aria-label="Pratinjau isi dokumen">
  {#if loading && !error}<p class="media-loading" role="status">Menyiapkan pratinjau…</p>{/if}
  {#if error}
    <div class="preview-empty" role="alert"><h3>Pratinjau tidak tersedia</h3><p>{error}</p></div>
  {:else if media === 'other'}
    <div class="preview-empty"><img src="/images/file-docx.svg" alt="" width="65" height="76" /><h3>Buka dengan aplikasi dokumen</h3><p>Pratinjau untuk jenis berkas ini belum tersedia. File lokal dapat dibuka dengan aplikasi yang mendukung formatnya.</p></div>
  {:else if source}
    {#if media === 'pdf'}
      <iframe src={source} title={`Pratinjau PDF ${file.name}`} onload={() => loading = false} onerror={failed}></iframe>
    {:else if media === 'image'}
      <img class="image-preview" src={source} alt={localFile ? `Isi ${file.name}` : 'Dokumen identitas contoh tanpa data pasien asli'} onload={() => loading = false} onerror={failed} />
    {:else if media === 'video'}
      <video controls playsinline preload="metadata" src={source} onloadedmetadata={() => loading = false} onerror={failed}><track kind="captions" label="Keterangan" srclang="id" src={localFile ? undefined : '/examples/video-example.vtt'} /></video>
    {/if}
  {/if}
</section>

<div class="preview-actions">
  {#if localFile && source}<a class="patient-button" href={source} download={file.name}>Unduh salinan lokal</a><p>Salinan berasal dari file yang Anda pilih, belum dari penyimpanan akun.</p>
  {:else}<button class="patient-button" type="button" disabled>Unduh dokumen</button><p>Ini adalah isi dokumen contoh. Bukan berkas yang tersimpan di akun Anda.</p>{/if}
</div>

<style>
  .preview { position: relative; min-height: 180px; margin-top: 26px; border: 1px solid #d1d8e0; border-radius: 14px; background: #f3f6fb; overflow: hidden; }
  .media-loading { padding: 14px 20px; margin: 0; font-size: 12px; }
  iframe { display: block; width: 100%; height: 430px; border: 0; }
  .image-preview { display: block; width: 100%; max-height: 430px; object-fit: contain; }
  video { display: block; width: 100%; max-height: 430px; background: #111827; }
  .preview-empty { padding: 36px 24px; text-align: center; }
  .preview-empty img { margin-bottom: 16px; }
  h3 { margin: 0; font-size: 17px; font-weight: 550; }
  .preview-empty p { margin: 12px auto 0; max-width: 45ch; font-size: 12px; line-height: 1.8; }
  .preview-actions { margin-top: 24px; }
  .preview-actions a { text-decoration: none; }
  .preview-actions > p { font-size: 11px; line-height: 1.8; margin: 12px 0 0; }
  @media (max-width: 480px) { iframe { height: 350px; } }
</style>
