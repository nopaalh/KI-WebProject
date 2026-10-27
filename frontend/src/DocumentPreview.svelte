<script lang="ts">
  import {
    ApiError,
    apiDecryptFile,
    apiGetFileMetadata,
    formatNsToMs,
    sanitizeDownloadFilename,
    type ApiFileRecord,
    type CipherAlgorithm,
  } from './api'

  let {
    file,
    localFile = null,
    remoteFile = null,
    onRemoteFileUpdated,
    onUnauthorized,
  }: {
    file: { name: string; kind: string; category: string }
    localFile?: File | null
    remoteFile?: ApiFileRecord | null
    onRemoteFileUpdated?: (updated: ApiFileRecord) => void
    onUnauthorized?: () => void
  } = $props()

  let source = $state('')
  let loading = $state(false)
  let error = $state('')
  let downloading = $state(false)
  let downloadMessage = $state('')
  let downloadError = $state('')
  let selectedAlgorithm = $state<CipherAlgorithm>('aes')

  const media = $derived(
    file.kind === 'PDF'
      ? 'pdf'
      : file.category === 'Foto'
        ? 'image'
        : file.category === 'Video'
          ? 'video'
          : 'other',
  )

  const availableAlgorithms = $derived.by<CipherAlgorithm[]>(() => {
    if (!remoteFile) return ['aes']
    if (Array.isArray(remoteFile.algorithms) && remoteFile.algorithms.length > 0) {
      return remoteFile.algorithms.filter(
        (alg): alg is CipherAlgorithm => alg === 'aes' || alg === 'des' || alg === 'rc4',
      )
    }
    return remoteFile.cipher_set_version === 1 ? ['aes', 'des', 'rc4'] : ['aes']
  })

  const isLegacyAesOnly = $derived(
    remoteFile !== null &&
      (remoteFile.cipher_set_version === 0 ||
        (!availableAlgorithms.includes('des') && !availableAlgorithms.includes('rc4'))),
  )

  $effect(() => {
    if (!availableAlgorithms.includes(selectedAlgorithm)) {
      selectedAlgorithm = availableAlgorithms[0] ?? 'aes'
    }
  })

  $effect(() => {
    const selected = localFile
    const remote = remoteFile
    const kind = media
    let active = true
    let url = ''
    const controller = new AbortController()

    source = ''
    error = ''
    downloadMessage = ''
    downloadError = ''

    if (selected) {
      loading = kind !== 'other'
      void (async () => {
        try {
          if (kind === 'pdf' && (await selected.slice(0, 5).text()) !== '%PDF-') {
            throw new Error('Invalid PDF')
          }
          if (!active) return
          url = URL.createObjectURL(
            kind === 'pdf' ? new Blob([selected], { type: 'application/pdf' }) : selected,
          )
          source = url
        } catch {
          if (active) {
            error = 'File ini tidak dapat ditampilkan. Pilih file lain.'
            loading = false
          }
        }
      })()
    } else if (remote) {
      if (kind === 'other') {
        loading = false
      } else {
        loading = true
        void (async () => {
          try {
            const { blob, decryptTimeNs, algorithm } = await apiDecryptFile(
              remote.id,
              'aes',
              controller.signal,
            )
            if (!active) return
            const mimeType =
              kind === 'pdf'
                ? 'application/pdf'
                : remote.original_type || blob.type || 'application/octet-stream'
            url = URL.createObjectURL(new Blob([blob], { type: mimeType }))
            source = url
            if (decryptTimeNs !== null && onRemoteFileUpdated) {
              onRemoteFileUpdated({
                ...remote,
                aes_decrypt_time_ns:
                  algorithm === 'aes' ? decryptTimeNs : remote.aes_decrypt_time_ns,
              })
            }
          } catch (err) {
            if (!active) return
            if (err instanceof DOMException && err.name === 'AbortError') return
            loading = false
            if (err instanceof ApiError) {
              if (err.status === 401) {
                onUnauthorized?.()
                return
              }
              if (err.status === 403) {
                error = 'Anda tidak memiliki izin untuk membuka berkas ini.'
              } else if (err.status === 404) {
                error = 'Berkas tidak ditemukan atau tidak tersedia untuk akun ini.'
              } else if (err.status === 409) {
                error = 'Representasi cipher yang diminta tidak tersedia untuk berkas lama ini.'
              } else if (err.isNetworkError) {
                error = 'Gagal terhubung ke server saat memuat pratinjau.'
              } else {
                error = 'Pratinjau tidak dapat didekripsi dari server.'
              }
            } else {
              error = 'Pratinjau tidak dapat dimuat.'
            }
          }
        })()
      }
    } else {
      loading = false
    }

    return () => {
      active = false
      controller.abort()
      if (url) URL.revokeObjectURL(url)
    }
  })

  function failed() {
    loading = false
    error = 'Berkas ini tidak dapat ditampilkan langsung di browser. Silakan unduh untuk membukanya di perangkat Anda.'
  }

  async function handleDownload() {
    if (!remoteFile || downloading) return
    downloading = true
    downloadError = ''
    downloadMessage = ''
    let objectUrl = ''
    try {
      const { blob, decryptTimeNs, algorithm } = await apiDecryptFile(
        remoteFile.id,
        selectedAlgorithm,
      )
      objectUrl = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = objectUrl
      link.download = sanitizeDownloadFilename(remoteFile.original_name || file.name)
      document.body.append(link)
      link.click()
      link.remove()
      setTimeout(() => {
        if (objectUrl) URL.revokeObjectURL(objectUrl)
      }, 1000)

      try {
        const refreshed = await apiGetFileMetadata(remoteFile.id)
        onRemoteFileUpdated?.(refreshed)
      } catch {
        if (decryptTimeNs !== null && onRemoteFileUpdated) {
          const updated: ApiFileRecord = { ...remoteFile }
          if (algorithm === 'aes') updated.aes_decrypt_time_ns = decryptTimeNs
          if (algorithm === 'des') updated.des_decrypt_time_ns = decryptTimeNs
          if (algorithm === 'rc4') updated.rc4_decrypt_time_ns = decryptTimeNs
          onRemoteFileUpdated(updated)
        }
      }

      const durationLabel =
        decryptTimeNs !== null ? ` (${formatNsToMs(decryptTimeNs)})` : ''
      downloadMessage = `Unduhan melalui dekripsi ${algorithm.toUpperCase()} selesai${durationLabel}.`
    } catch (err) {
      if (objectUrl) URL.revokeObjectURL(objectUrl)
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 403) {
          downloadError = 'Akses ditolak (403). Anda tidak berwenang mengunduh berkas ini.'
        } else if (err.status === 404) {
          downloadError = 'Berkas tidak ditemukan (404).'
        } else if (err.status === 409) {
          downloadError =
            'Cipher yang dipilih tidak tersedia untuk berkas lama (AES-only). Pilih AES atau unggah ulang berkas asli.'
        } else if (err.status === 400) {
          downloadError = 'Pilihan algoritma dekripsi tidak valid.'
        } else if (err.isNetworkError) {
          downloadError = 'Gagal terhubung ke server. Periksa koneksi Anda.'
        } else {
          downloadError = 'Gagal memproses dekripsi berkas di server.'
        }
      } else {
        downloadError = 'Terjadi kesalahan saat mengunduh berkas.'
      }
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
    <div class="preview-empty"><img src="/images/file-docx.svg" alt="" width="65" height="76" /><h3>Buka dengan aplikasi dokumen</h3><p>Pratinjau langsung belum tersedia untuk jenis berkas ini. Silakan unduh berkas untuk membukanya di perangkat Anda.</p></div>
  {:else if source}
    {#if media === 'pdf'}
      <iframe src={source} title={`Pratinjau PDF ${file.name}`} onload={() => loading = false} onerror={failed}></iframe>
    {:else if media === 'image'}
      <img class="image-preview" src={source} alt={`Pratinjau ${file.name}`} onload={() => loading = false} onerror={failed} />
    {:else if media === 'video'}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video controls playsinline preload="metadata" src={source} onloadedmetadata={() => loading = false} onerror={failed}></video>
    {/if}
  {/if}
</section>

{#if remoteFile}
  <section class="cipher-meta" aria-label="Metadata pemrosesan cipher">
    <div class="cipher-meta-header">
      <h3>Representasi cipher tercatat di metadata</h3>
      <span class="cipher-version">
        {isLegacyAesOnly ? 'Versi 0 (Legacy AES-only)' : `Versi ${remoteFile.cipher_set_version} (${availableAlgorithms.map(a => a.toUpperCase()).join(', ')})`}
      </span>
    </div>
    {#if isLegacyAesOnly}
      <p class="legacy-note">Berkas lama ini hanya memiliki representasi AES. Untuk ketersediaan DES dan RC4, unggah ulang berkas asli.</p>
    {/if}
    <div class="timing-grid">
      <div class="timing-card">
        <strong>AES</strong>
        <span>Enkripsi: {formatNsToMs(remoteFile.aes_encrypt_time_ns)}</span>
        <span>Dekripsi terakhir: {formatNsToMs(remoteFile.aes_decrypt_time_ns)}</span>
      </div>
      <div class="timing-card" class:unavailable={!availableAlgorithms.includes('des')}>
        <strong>DES</strong>
        {#if availableAlgorithms.includes('des')}
          <span>Enkripsi: {formatNsToMs(remoteFile.des_encrypt_time_ns)}</span>
          <span>Dekripsi terakhir: {formatNsToMs(remoteFile.des_decrypt_time_ns)}</span>
        {:else}
          <span>Tidak tersedia pada berkas lama</span>
        {/if}
      </div>
      <div class="timing-card" class:unavailable={!availableAlgorithms.includes('rc4')}>
        <strong>RC4</strong>
        {#if availableAlgorithms.includes('rc4')}
          <span>Enkripsi: {formatNsToMs(remoteFile.rc4_encrypt_time_ns)}</span>
          <span>Dekripsi terakhir: {formatNsToMs(remoteFile.rc4_decrypt_time_ns)}</span>
        {:else}
          <span>Tidak tersedia pada berkas lama</span>
        {/if}
      </div>
    </div>
  </section>
{/if}

<div class="preview-actions">
  {#if localFile && source}
    <a class="patient-button" href={source} download={sanitizeDownloadFilename(file.name)}>Unduh salinan lokal</a>
    <p>Salinan berasal dari berkas yang sedang Anda pilih di perangkat.</p>
  {:else if remoteFile}
    <div class="decrypt-controls">
      <label class="algo-select">
        <span>Algoritma dekripsi unduhan</span>
        <select bind:value={selectedAlgorithm} disabled={downloading}>
          {#each (['aes', 'des', 'rc4'] as const) as alg}
            <option value={alg} disabled={!availableAlgorithms.includes(alg)}>
              {alg.toUpperCase()}{!availableAlgorithms.includes(alg) ? ' (tidak tersedia)' : ''}
            </option>
          {/each}
        </select>
      </label>
      <button class="patient-button" type="button" onclick={handleDownload} disabled={downloading}>
        {downloading ? 'Mendekripsi & mengunduh…' : `Unduh (${selectedAlgorithm.toUpperCase()})`}
      </button>
    </div>
    {#if downloadMessage}<p class="download-feedback success" role="status">{downloadMessage}</p>{/if}
    {#if downloadError}<p class="download-feedback error" role="alert">{downloadError}</p>{/if}
  {:else}
    <button class="patient-button" type="button" disabled>Unduh dokumen</button>
    <p>Berkas belum tersedia di server.</p>
  {/if}
</div>

<style>
  .preview { position: relative; min-height: 180px; margin-top: 26px; border: 1px solid #d1d8e0; border-radius: 14px; background: #f3f6fb; overflow: hidden; }
  .media-loading { padding: 14px 20px; margin: 0; font-size: 12px; }
  iframe { display: block; width: 100%; height: 430px; border: 0; }
  .image-preview { display: block; width: 100%; max-height: 430px; object-fit: contain; }
  video { display: block; width: 100%; max-height: 430px; background: #111827; }
  .preview-empty { padding: 36px 24px; text-align: center; }
  .preview-empty img { margin-bottom: 16px; }
  h3 { margin: 0; font-size: 15px; font-weight: 550; }
  .preview-empty p { margin: 12px auto 0; max-width: 45ch; font-size: 12px; line-height: 1.8; }
  .cipher-meta { margin-top: 20px; padding: 16px; border: 1px solid #dce3ec; border-radius: 14px; background: #f9fbfd; }
  .cipher-meta-header { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 8px; margin-bottom: 12px; }
  .cipher-version { font-size: 11px; padding: 4px 9px; border-radius: 7px; background: #eef4fc; color: #0064cb; font-weight: 500; }
  .legacy-note { margin: 0 0 12px; font-size: 11px; color: #5b6470; line-height: 1.6; }
  .timing-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
  .timing-card { padding: 10px 12px; border: 1px solid #e1e6ec; border-radius: 10px; background: #fff; font-size: 11px; display: grid; gap: 4px; }
  .timing-card.unavailable { opacity: .65; background: #f5f7f9; }
  .timing-card strong { font-size: 12px; }
  .preview-actions { margin-top: 24px; }
  .preview-actions a { text-decoration: none; }
  .preview-actions > p { font-size: 11px; line-height: 1.8; margin: 12px 0 0; }
  .decrypt-controls { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 14px; }
  .algo-select { display: grid; gap: 6px; font-size: 11px; font-weight: 500; }
  .algo-select select { height: 44px; padding: 0 14px; border: 1px solid #cbd3dc; border-radius: 12px; background: #fff; font: inherit; font-size: 12px; color: inherit; }
  .download-feedback { font-size: 12px; margin: 12px 0 0; line-height: 1.6; }
  .download-feedback.success { color: #2f5e2a; }
  .download-feedback.error { color: #9f3a30; }
  @media (max-width: 600px) {
    .timing-grid { grid-template-columns: minmax(0, 1fr); }
    .decrypt-controls { flex-direction: column; align-items: stretch; }
  }
  @media (max-width: 480px) { iframe { height: 350px; } }
</style>
