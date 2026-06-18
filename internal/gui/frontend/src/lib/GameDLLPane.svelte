<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'

  export let game
  export let desktop

  const dispatch = createEventDispatcher()

  let dllUpdates = []
  let hasBackup = false
  let updatingDLLs = false
  let restoringDLLs = false
  let installOpen = false
  let installStep = 'type'
  let installTypes = []
  let installVersions = []
  let selectedInstallType = ''
  let installingDLL = false
  let installError = ''
  let dllProgressStage = ''
  let unsubscribeDllProgress = null

  $: hasUpdates = dllUpdates.some(d => d.hasUpdate)

  let lastGameId = null

  $: if (game && game.appId !== lastGameId) {
    lastGameId = game.appId
    closeInstallWizard()
    void checkDLLUpdates()
  }

  onMount(async () => {
    unsubscribeDllProgress = desktop.SubscribeDLLProgress((stage) => {
      dllProgressStage = stage || ''
    })
    if (game) {
      await checkDLLUpdates()
    }
  })

  onDestroy(() => {
    unsubscribeDllProgress?.()
  })

  async function checkDLLUpdates() {
    if (!game) {
      dllUpdates = []
      hasBackup = false
      return
    }
    dllUpdates = await desktop.CheckDLLUpdates(game.appId) || []
    hasBackup = await desktop.HasDLLBackup(game.appId)
  }

  function formatError(e) {
    if (typeof e === 'string') return e
    if (e?.message) return e.message
    return String(e)
  }

  function clearDllProgress() {
    dllProgressStage = ''
  }

  async function refreshGameDetails() {
    if (!game) {
      return
    }
    const updated = await desktop.GetGame(game.appId)
    if (updated) {
      game = updated
      dispatch('gameUpdate', updated)
    }
  }

  function closeInstallWizard() {
    installOpen = false
    installStep = 'type'
    installTypes = []
    installVersions = []
    selectedInstallType = ''
    installError = ''
    installingDLL = false
  }

  async function openInstallWizard() {
    if (!game) {
      return
    }
    installOpen = true
    installStep = 'type'
    installError = ''
    selectedInstallType = ''
    installVersions = []
    installingDLL = false
    try {
      installTypes = await desktop.ListDLLInstallTypes(game.appId)
      if (!installTypes || installTypes.length === 0) {
        installError = 'No supported DLL types detected for this game.'
      }
    } catch (e) {
      installError = formatError(e)
    }
  }

  async function selectInstallType(type) {
    if (!type) {
      return
    }
    selectedInstallType = type
    installStep = 'version'
    installVersions = []
    installError = ''
    try {
      installVersions = await desktop.ListDLLVersions(type)
      if (!installVersions || installVersions.length === 0) {
        installError = `No versions available for ${formatInstallType(type)}.`
      }
    } catch (e) {
      installError = formatError(e)
    }
  }

  async function selectInstallVersion(version) {
    if (!game || !selectedInstallType) {
      return
    }
    installingDLL = true
    installError = ''
    try {
      await desktop.InstallDLL(game.appId, selectedInstallType, version)
      await refreshGameDetails()
      await checkDLLUpdates()
      dispatch('success', 'DLL installed!')
      closeInstallWizard()
    } catch (e) {
      installError = formatError(e)
    } finally {
      clearDllProgress()
      installingDLL = false
    }
  }

  function formatInstallType(type) {
    const labels = {
      dlss: 'DLSS',
      dlssg: 'DLSS-G',
      dlssd: 'DLSS-D',
      xess: 'XeSS',
      fsr: 'FSR'
    }
    return labels[type] || type.toUpperCase()
  }

  function formatInstallVersion(version, index) {
    if (!version) {
      return 'Unknown'
    }
    if (index === 0) {
      return `${version} (latest)`
    }
    return version
  }

  async function updateDLLs() {
    updatingDLLs = true
    try {
      await desktop.UpdateDLLs(game.appId)
      await refreshGameDetails()
      await checkDLLUpdates()
      dispatch('success', 'DLLs updated!')
    } catch (e) {
      dispatch('error', 'Failed to update: ' + formatError(e))
    } finally {
      clearDllProgress()
      updatingDLLs = false
    }
  }

  async function restoreDLLs() {
    restoringDLLs = true
    try {
      await desktop.RestoreDLLs(game.appId)
      await refreshGameDetails()
      await checkDLLUpdates()
      dispatch('success', 'DLLs restored!')
    } catch (e) {
      dispatch('error', 'Failed to restore: ' + formatError(e))
    } finally {
      clearDllProgress()
      restoringDLLs = false
    }
  }
</script>

<div class="section">
  <h2>DLL versions</h2>
  <div class="dll-table">
    <div class="dll-row dll-header">
      <span class="dll-cell">DLSS</span>
      <span class="dll-cell">DLSS-G</span>
      <span class="dll-cell">DLSS-D</span>
      <span class="dll-cell">XESS</span>
      <span class="dll-cell">FSR</span>
    </div>
    <div class="dll-row">
      <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'dlss')?.version || '-'}</span>
      <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'dlssg')?.version || '-'}</span>
      <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'dlssd')?.version || '-'}</span>
      <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'xess')?.version || '-'}</span>
      <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'fsr')?.version || '-'}</span>
    </div>
  </div>
  <div class="dll-actions">
    {#if hasUpdates}
      <button class="update-btn" on:click={updateDLLs} disabled={updatingDLLs}>
        {updatingDLLs ? 'Updating...' : 'Update all DLLs'}
      </button>
    {/if}
    <button class="install-btn" on:click={openInstallWizard} disabled={installingDLL}>
      {installingDLL ? 'Installing...' : 'Install DLL'}
    </button>
    {#if hasBackup}
      <button class="restore-btn" on:click={restoreDLLs} disabled={restoringDLLs}>
        {restoringDLLs ? 'Restoring...' : 'Restore original DLLs'}
      </button>
    {/if}
    {#if hasBackup}
      <span class="backup-hint">Backup available</span>
    {/if}
    {#if dllProgressStage && (updatingDLLs || restoringDLLs || installingDLL)}
      <span class="dll-progress">{dllProgressStage}…</span>
    {/if}
  </div>
</div>

{#if installOpen}
  <button type="button" class="install-overlay" on:click={closeInstallWizard} aria-label="Close install"></button>
  <div class="install-panel" role="dialog" aria-modal="true" aria-label="Install DLL">
    <div class="install-header">
      <div class="install-title">Install DLL</div>
      <button class="install-close" on:click={closeInstallWizard}>Close</button>
    </div>
    {#if installError}
      <div class="install-message" data-type="error">{installError}</div>
    {/if}
    {#if installStep === 'type'}
      <div class="install-step">Select DLL type</div>
      <div class="install-options">
        {#each installTypes as type}
          <button class="install-option" on:click={() => selectInstallType(type)} disabled={installingDLL}>
            {formatInstallType(type)}
          </button>
        {/each}
      </div>
    {:else if installStep === 'version'}
      <div class="install-step">Select version</div>
      <div class="install-options">
        {#each installVersions as version, index}
          <button class="install-option" on:click={() => selectInstallVersion(version)} disabled={installingDLL}>
            {formatInstallVersion(version, index)}
          </button>
        {/each}
      </div>
      <button class="install-back" on:click={() => (installStep = 'type')} disabled={installingDLL}>Back</button>
    {/if}
    {#if installingDLL}
      <div class="install-status">Installing DLL...</div>
    {/if}
  </div>
{/if}

<style>
  h2 {
    font-size: 0.85rem;
    color: var(--accent-secondary);
    margin-bottom: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .section {
    margin-bottom: 1.5rem;
  }

  .dll-table {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.5rem 0.75rem;
    background-color: var(--bg-secondary);
  }

  .dll-row {
    display: grid;
    grid-template-columns: repeat(5, minmax(80px, 1fr));
    gap: 0.5rem;
    padding: 0.35rem 0;
    font-size: 0.85rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .dll-header {
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 0.7rem;
  }

  .dll-header .dll-cell {
    color: var(--text-dim);
  }

  .dll-cell {
    color: var(--accent-secondary);
  }

  .dll-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
    align-items: center;
    flex-wrap: wrap;
  }

  .backup-hint {
    color: var(--text-dim);
    font-size: 0.75rem;
  }

  .dll-progress {
    color: var(--text-dim);
    font-size: 0.75rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    border: none;
    padding: 0;
    z-index: 3000;
  }

  .install-panel {
    position: fixed;
    top: 6rem;
    right: 2rem;
    width: min(420px, 92vw);
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 1rem;
    z-index: 3001;
  }

  .install-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .install-title {
    font-size: 0.9rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-close {
    border: none;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.7rem;
  }

  .install-close:hover {
    color: var(--text-primary);
  }

  .install-step {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    margin-bottom: 0.5rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-options {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .install-option {
    text-align: left;
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    padding: 0.4rem 0.6rem;
    cursor: pointer;
    font-size: 0.8rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-option:hover {
    border-color: var(--border-focus);
  }

  .install-message {
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border-default);
    border-radius: 0;
    font-size: 0.75rem;
    margin-bottom: 0.6rem;
    color: var(--error);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-back {
    margin-top: 0.75rem;
    border: none;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.75rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .install-back:hover {
    color: var(--text-primary);
  }

  .install-status {
    margin-top: 0.75rem;
    font-size: 0.75rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  @media (max-width: 720px) {
    .install-panel {
      right: 1rem;
      left: 1rem;
      top: 5rem;
    }
  }

  .update-btn,
  .restore-btn,
  .install-btn {
    padding: 0.4rem 0.9rem;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .update-btn {
    background-color: var(--success);
    color: black;
  }

  .update-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .install-btn {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .install-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .restore-btn {
    background-color: var(--border-default);
    color: var(--text-primary);
  }

  .restore-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .update-btn:disabled, .restore-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
