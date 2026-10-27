<script lang="ts">
  import { ApiError, apiDecryptFile, apiGetFileMetadata, formatNsToMs, sanitizeDownloadFilename, type ApiFileRecord, type CipherAlgorithm } from './api'

  let { file, localFile = null, remoteFile = null, onRemoteFileUpdated, onUnauthorized }: { file: { name: string; kind: string; category: string }; localFile?: File | null; remoteFile?: ApiFileRecord | null; onRemoteFileUpdated?: (updated: ApiFileRecord) => void; onUnauthorized?: () => void } = $props()
  let source = $state('')
  let loading = $state(true)
  let error = $state('')
  let downloading = $state(false)
  let downloadNotice = $state('')
  let selectedAlgorithm = $state<CipherAlgorithm>('aes')
  const media = $derived(file.kind === 'PDF' ? 'pdf' : file.category === 'Foto' ? 'image' : file.category === 'Video' ? 'video' : 'other')
  const availableAlgorithms = $derived.by<CipherAlgorithm[]>(() => {
    if (!remoteFile) return ['aes']
    const valid = (remoteFile.algorithms ?? []).filter((a): a is CipherAlgorithm => a === 'aes' || a === 'des' || a === 'rc4')
    return valid.length ? valid : remoteFile.cipher_set_version === 1 ? ['aes', 'des', 'rc4'] : ['aes']
  })
  const isLegacyAesOnly = $derived(remoteFile !== null && (remoteFile.cipher_set_version === 0 || (!availableAlgorithms.includes('des') && !availableAlgorithms.includes('rc4'))))

  $effect(() => { if (!availableAlgorithms.includes(selectedAlgorithm)) selectedAlgorithm = availableAlgorithms[0] ?? 'aes' })

  $effect(() => {
    const selected = localFile
    const remote = remoteFile
    const kind = media
    let active = true
    let url = ''
    const controller = new AbortController()
    source = ''
    error = ''
    downloadNotice = ''
    loading = kind !== 'other'
    if (selected) {
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
    } else if (remote && kind !== 'other') {
      void (async () => {
        try {
          const { blob, decryptTimeNs, algorithm } = await apiDecryptFile(remote.id, 'aes', controller.signal)
          if (!active) return
          url = URL.createObjectURL(new Blob([blob], { type: kind === 'pdf' ? 'application/pdf' : remote.original_type || blob.type || 'application/octet-stream' }))
          source = url
          if (decryptTimeNs !== null && algorithm === 'aes') onRemoteFileUpdated?.({ ...remote, aes_decrypt_time_ns: decryptTimeNs })
        } catch (err) {
          if (!active || (err instanceof DOMException && err.name === 'AbortError')) return
          loading = false
          if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
          error = err instanceof ApiError && err.status === 403 ? 'Akses ditolak untuk berkas ini.' : 'Pratinjau tidak dapat dimuat dari server.'
        }
      })()
    } else {
      loading = false
    }
    return () => { active = false; controller.abort(); if (url) URL.revokeObjectURL(url) }
  })

  function failed() {
    loading = false
    error = 'File ini tidak dapat ditampilkan oleh browser. Anda dapat mengunduh berkasnya di perangkat.'
  }

  async function downloadRemote() {
    if (!remoteFile || downloading) return
    downloading = true
    downloadNotice = ''
    let url = ''
    try {
      const { blob, decryptTimeNs, algorithm } = await apiDecryptFile(remoteFile.id, selectedAlgorithm)
      url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = sanitizeDownloadFilename(remoteFile.original_name || file.name)
      document.body.append(link)
      link.click()
      link.remove()
      setTimeout(() => { if (url) URL.revokeObjectURL(url) }, 1000)
      try { onRemoteFileUpdated?.(await apiGetFileMetadata(remoteFile.id)) } catch {}
      downloadNotice = `Unduhan ${algorithm.toUpperCase()} selesai${decryptTimeNs !== null ? ` (${formatNsToMs(decryptTimeNs)})` : ''}.`
    } catch (err) {
      if (url) URL.revokeObjectURL(url)
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      downloadNotice = err instanceof ApiError && err.status === 409 ? 'Cipher tersebut tidak tersedia untuk berkas lama (AES-only).' : 'Gagal mendekripsi dan mengunduh berkas.'
    } finally {
      downloading = false
    }
  }
</script>

<section class="preview" aria-label="Pratinjau isi dokumen">
  {#if loading && !error}<p class="media-loading" role="status">Menyiapkan pratinjau…</p>{/if}
  {#if error}
    <div class="preview-empty" role="alert"><h3>Pratinjau tidak tersedia</h3><p>{error}</p></div>
  {:else if media === 'other'}
    <div class="preview-empty"><img src="/images/file-docx.svg" alt="" width="65" height="76" /><h3>Buka dengan aplikasi dokumen</h3><p>Pratinjau untuk jenis berkas ini belum tersedia. Unduh berkas untuk membukanya di perangkat Anda.</p></div>
  {:else if source}
    {#if media === 'pdf'}
      <iframe src={source} title={`Pratinjau PDF ${file.name}`} onload={() => loading = false} onerror={failed}></iframe>
    {:else if media === 'image'}
      <img class="image-preview" src={source} alt={`Isi ${file.name}`} onload={() => loading = false} onerror={failed} />
    {:else if media === 'video'}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video controls playsinline preload="metadata" src={source} onloadedmetadata={() => loading = false} onerror={failed}></video>
    {/if}
  {/if}
</section>

<div class="preview-actions">
  {#if localFile && source}<a class="patient-button" href={source} download={sanitizeDownloadFilename(file.name)}>Unduh salinan lokal</a><p>Salinan berasal dari file yang Anda pilih, belum dari penyimpanan akun.</p>
  {:else if remoteFile}
    <div class="decrypt-row">
      <label class="algo-picker"><span>Representasi cipher</span><select bind:value={selectedAlgorithm} disabled={downloading}>{#each (['aes', 'des', 'rc4'] as const) as alg}<option value={alg} disabled={!availableAlgorithms.includes(alg)}>{alg.toUpperCase()}{!availableAlgorithms.includes(alg) ? ' (tidak tersedia)' : ''}</option>{/each}</select></label>
      <button class="patient-button" type="button" disabled={downloading} onclick={downloadRemote}>{downloading ? 'Mendekripsi…' : `Unduh (${selectedAlgorithm.toUpperCase()})`}</button>
    </div>
    <p>{isLegacyAesOnly ? 'Berkas lama (AES-only).' : `Tersedia cipher: ${availableAlgorithms.map(a => a.toUpperCase()).join(', ')}.`} Waktu dekripsi terakhir — AES: {formatNsToMs(remoteFile.aes_decrypt_time_ns)}, DES: {formatNsToMs(remoteFile.des_decrypt_time_ns)}, RC4: {formatNsToMs(remoteFile.rc4_decrypt_time_ns)}.</p>
    {#if downloadNotice}<p role="status">{downloadNotice}</p>{/if}
  {:else}<button class="patient-button" type="button" disabled>Unduh dokumen</button><p>Berkas belum tersedia di server.</p>{/if}
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
  .decrypt-row { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; }
  .algo-picker { display: grid; gap: 6px; font-size: 11px; }
  .algo-picker select { height: 44px; padding: 0 12px; border: 1px solid #d1d8e0; border-radius: 10px; background: #fff; font: inherit; font-size: 12px; }
  @media (max-width: 480px) { iframe { height: 350px; } }
</style>
