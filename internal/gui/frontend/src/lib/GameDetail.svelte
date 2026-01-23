<script>
  import { onMount, createEventDispatcher, tick } from 'svelte'
  import {
    CheckDLLUpdates,
    GetDefaultProfile,
    GetGame,
    GetProfile,
    HasDLLBackup,
    InstallDLL,
    LaunchGame,
    ListDLLInstallTypes,
    ListDLLVersions,
    RestoreDLLs,
    SaveDefaultProfile,
    SaveProfile,
    UpdateDLLs
  } from '../../wailsjs/go/gui/App'
  import Dropdown from './Dropdown.svelte'

  export let game
  export let profileMode = 'game'

  const dispatch = createEventDispatcher()

  let profile = null
  let saving = false
  let message = ''
  let messageType = 'info'
  let dllUpdates = []
  let hasBackup = false
  let updatingDLLs = false
  let restoringDLLs = false
  let launching = false
  let messageTimer
  let installOpen = false
  let installStep = 'type'
  let installTypes = []
  let installVersions = []
  let selectedInstallType = ''
  let installingDLL = false
  let installError = ''
  let root

  const srModeOptions = [
    { value: '', label: '(default)' },
    { value: 'off', label: 'Off' },
    { value: 'ultra_performance', label: 'Ultra performance' },
    { value: 'performance', label: 'Performance' },
    { value: 'balanced', label: 'Balanced' },
    { value: 'quality', label: 'Quality' },
    { value: 'dlaa', label: 'DLAA' }
  ]
  const srPresetOptions = [
    { value: '', label: '(default)' },
    { value: 'A', label: 'A' },
    { value: 'B', label: 'B' },
    { value: 'C', label: 'C' },
    { value: 'D', label: 'D' },
    { value: 'E', label: 'E' },
    { value: 'F', label: 'F' },
    { value: 'J', label: 'J' },
    { value: 'K', label: 'K' },
    { value: 'L', label: 'L' },
    { value: 'M', label: 'M' }
  ]
  const multiFrameOptions = [
    { value: 0, label: '(default)' },
    { value: 1, label: '1' },
    { value: 2, label: '2' },
    { value: 3, label: '3' },
    { value: 4, label: '4' }
  ]
  const powerMizerOptions = [
    { value: '', label: '(default)' },
    { value: 'adaptive', label: 'Adaptive' },
    { value: 'max', label: 'Max performance' }
  ]
  const frameGenerationOptions = [
    { value: '(default)', label: '(default)' },
    { value: 'true', label: 'true' },
    { value: 'false', label: 'false' }
  ]

  const emptyProfile = () => ({
    srMode: '',
    srPreset: '',
    srOverride: false,
    fgEnabled: false,
    fgOverride: false,
    multiFrame: 0,
    indicator: false,
    shaderCache: false,
    threadedOptimization: false,
    powerMizer: '',
    enableHdr: false,
    enableWayland: false,
    enableNgxUpdater: false,
    backupOnLaunch: false,
    inheritedFromDefault: false
  })

  onMount(async () => {
    await loadProfile()
    if (profileMode === 'game' && game) {
      await checkDLLUpdates()
    }
  })

  let lastGameId = null
  let lastProfileMode = profileMode
  let frameGenerationMode = '(default)'

  $: if (profileMode !== lastProfileMode) {
    lastProfileMode = profileMode
    lastGameId = null
    closeInstallWizard()
    void loadProfile()
    if (profileMode === 'game' && game) {
      void checkDLLUpdates()
    }
  }

  $: if (profileMode === 'game' && game && game.appId !== lastGameId) {
    lastGameId = game.appId
    closeInstallWizard()
    void loadProfile()
    void checkDLLUpdates()
  }

  $: if (profile) {
    frameGenerationMode = profile.fgOverride
      ? (profile.fgEnabled ? 'true' : 'false')
      : '(default)'
  }

  async function loadProfile() {
    if (profileMode === 'default') {
      profile = await GetDefaultProfile()
      if (!profile) {
        profile = emptyProfile()
      }
      return
    }
    if (!game) {
      profile = null
      return
    }
    profile = await GetProfile(game.appId)
    if (!profile) {
      profile = emptyProfile()
    }
  }


  async function checkDLLUpdates() {
    if (!game) {
      dllUpdates = []
      hasBackup = false
      return
    }
    dllUpdates = await CheckDLLUpdates(game.appId) || []
    hasBackup = await HasDLLBackup(game.appId)
  }

  function formatError(e) {
    if (typeof e === 'string') return e
    if (e?.message) return e.message
    return String(e)
  }

  function updateFrameGeneration(value) {
    if (!profile) {
      return
    }
    if (value === '(default)') {
      profile.fgOverride = false
      profile.fgEnabled = false
      return
    }
    profile.fgOverride = true
    profile.fgEnabled = value === 'true'
  }

  function clearMessageAfter(delay) {
    if (messageTimer) {
      clearTimeout(messageTimer)
    }
    messageTimer = setTimeout(() => {
      message = ''
      messageTimer = null
    }, delay)
  }

  function setMessage(nextMessage, type) {
    message = nextMessage
    messageType = type
    clearMessageAfter(3000)
  }

  async function save() {
    saving = true
    try {
      if (profileMode === 'default') {
        await SaveDefaultProfile(profile)
        setMessage('Default profile saved!', 'success')
      } else if (game) {
        await SaveProfile(game.appId, profile)
        profile.inheritedFromDefault = false
        await refreshGameDetails()
        setMessage('Profile saved!', 'success')
      }
    } catch (e) {
      setMessage('Failed to save: ' + formatError(e), 'error')
    }
    saving = false
  }

  async function refreshGameDetails() {
    if (!game) {
      return
    }
    const updated = await GetGame(game.appId)
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
      installTypes = await ListDLLInstallTypes(game.appId)
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
      installVersions = await ListDLLVersions(type)
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
      await InstallDLL(game.appId, selectedInstallType, version)
      await refreshGameDetails()
      await checkDLLUpdates()
      setMessage('DLL installed!', 'success')
      closeInstallWizard()
    } catch (e) {
      installError = formatError(e)
    }
    installingDLL = false
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
      await UpdateDLLs(game.appId)
      await refreshGameDetails()
      await checkDLLUpdates()
      setMessage('DLLs updated!', 'success')
    } catch (e) {
      setMessage('Failed to update: ' + formatError(e), 'error')
    }
    updatingDLLs = false
  }

  async function restoreDLLs() {
    restoringDLLs = true
    try {
      await RestoreDLLs(game.appId)
      await refreshGameDetails()
      await checkDLLUpdates()
      setMessage('DLLs restored!', 'success')
    } catch (e) {
      setMessage('Failed to restore: ' + formatError(e), 'error')
    }
    restoringDLLs = false
  }

  $: hasUpdates = dllUpdates.some(d => d.hasUpdate)

  async function launchGame() {
    if (!game) {
      return
    }
    launching = true
    try {
      await LaunchGame(game.appId)
      setMessage('Game launched!', 'success')
    } catch (e) {
      setMessage('Failed to launch: ' + formatError(e), 'error')
    }
    launching = false
  }

  export async function focusPrimary() {
    await tick()
    const focusTarget = root?.querySelector(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
    focusTarget?.focus()
  }
</script>

  <div class="detail" bind:this={root}>
    {#if profileMode === 'default'}
      <div class="default-header">
        <h1>Default profile</h1>
        <p class="default-note">Applies to games without their own profile.</p>
      </div>
    {:else if game}
      <div class="game-header">
        <div class="game-title">
          <h1>{game.name}</h1>
          <div class="info">
            <div class="row">
              <span class="label">App ID</span>
              <span class="value">{game.appId}</span>
            </div>
            <div class="row">
              <span class="label">Install dir</span>
              <span class="value">{game.installDir}</span>
            </div>
            {#if game.prefixPath}
              <div class="row">
                <span class="label">Prefix</span>
                <span class="value">{game.prefixPath}</span>
              </div>
            {/if}
          </div>
        </div>
        <button class="launch" on:click={launchGame} disabled={launching}>
          {launching ? 'Launching...' : '▶ Launch'}
        </button>
      </div>
    {/if}


    {#if profileMode === 'game' && game}
      <div class="section">
        <h2>DLL versions</h2>
        <div class="dll-table">
          <div class="dll-row dll-header">
            <span class="dll-cell">DLSS</span>
            <span class="dll-cell">DLSS-G</span>
            <span class="dll-cell">XESS</span>
            <span class="dll-cell">FSR</span>
          </div>
          <div class="dll-row">
            <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'dlss')?.version || '-'}</span>
            <span class="dll-cell">{game.dlls?.find(d => d.dllType === 'dlssg')?.version || '-'}</span>
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
    {/if}


  {#if profile}
    {#if profileMode === 'game' && profile.inheritedFromDefault}
      <p class="default-note">Using default profile values.</p>
    {/if}
    <div class="profile-grid">
      <div class="section boxed">
        <h2>DLSS settings</h2>

        <div class="form">
          <div class="field">
            <label for="srMode">Quality mode</label>
            <Dropdown
              bind:value={profile.srMode}
              options={srModeOptions}
            />
            <span class="hint">Resolution preset for DLSS super resolution.</span>
          </div>

          <div class="field">
            <label for="srPreset">DLSS preset</label>
            <Dropdown
              bind:value={profile.srPreset}
              options={srPresetOptions}
            />
            <span class="hint">A-F: CNN (DLSS 2/3), J-M: Transformer (DLSS 4/4.5)</span>
          </div>

          <div class="field checkbox">
            <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
            <label for="srOverride">Override (force DLSS even if unsupported)</label>
            <span class="hint">Use DLSS even if the game does not expose it.</span>
          </div>

          <div class="field checkbox">
            <input type="checkbox" id="indicator" bind:checked={profile.indicator} />
            <label for="indicator">Show DLSS indicator</label>
            <span class="hint">Display a small on-screen DLSS status overlay.</span>
          </div>

          <div class="field">
            <label for="fgEnabled">Frame generation</label>
            <Dropdown
              bind:value={frameGenerationMode}
              options={frameGenerationOptions}
              on:change={(event) => updateFrameGeneration(event.detail)}
            />
            <span class="hint">Generate extra frames for higher FPS.</span>
          </div>

          <div class="field">
            <label for="multiFrame">Multi-frame generation</label>
            <Dropdown
              bind:value={profile.multiFrame}
              options={multiFrameOptions}
            />
            <span class="hint">Extra frames to generate (0=off).</span>
          </div>
        </div>
      </div>

      <div class="section boxed">
        <h2>GPU settings</h2>

        <div class="form">
          <div class="field checkbox">
            <input type="checkbox" id="shaderCache" bind:checked={profile.shaderCache} />
            <label for="shaderCache">Shader cache</label>
            <span class="hint">Enable shader caching for faster reloads.</span>
          </div>

          <div class="field checkbox">
            <input type="checkbox" id="threadedOptimization" bind:checked={profile.threadedOptimization} />
            <label for="threadedOptimization">Threaded optimization</label>
            <span class="hint">Use multi-core rendering when supported.</span>
          </div>

          <div class="field">
            <label for="powerMizer">Power mode</label>
            <Dropdown
              bind:value={profile.powerMizer}
              options={powerMizerOptions}
            />
            <span class="hint">GPU power policy for the game.</span>
          </div>
        </div>
      </div>

      <div class="section boxed">
        <h2>Proton settings</h2>

        <div class="form">
          <div class="field checkbox">
            <input type="checkbox" id="enableHdr" bind:checked={profile.enableHdr} />
            <label for="enableHdr">HDR</label>
            <span class="hint">Enable HDR output for supported displays.</span>
          </div>

          <div class="field checkbox">
            <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
            <label for="enableWayland">Wayland</label>
            <span class="hint">Prefer native Wayland when available.</span>
          </div>

          <div class="field checkbox">
            <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
            <label for="enableNgxUpdater">NGX Updater</label>
            <span class="hint">Allow Proton to update DLSS DLLs.</span>
          </div>
        </div>
      </div>

      <div class="section boxed">
        <h2>Backup settings</h2>

        <div class="form">
          <div class="field checkbox">
            <input type="checkbox" id="backupOnLaunch" bind:checked={profile.backupOnLaunch} />
            <label for="backupOnLaunch">Save backup</label>
            <span class="hint">Backup saves when launching via Ludusavi.</span>
          </div>
        </div>
      </div>
    </div>

    <div class="actions">
      <button class="save" on:click={save} disabled={saving}>
        {saving ? 'Saving...' : profileMode === 'default' ? 'Save default profile' : 'Save profile'}
      </button>

      {#if message}
        <div class="message" data-type={messageType}>{message}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .detail > .section:last-of-type {
    margin-bottom: 0;
  }

  .detail h1 {
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .detail h2 {
    text-transform: uppercase;
  }

  .game-title {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    flex: 1;
    min-width: 260px;
  }

  .game-title h1 {
    margin: 0;
  }

  .launch {
    padding: 0.45rem 1.4rem;
    border: none;
    border-radius: 0;
    background-color: var(--success);
    color: black;
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    align-self: flex-start;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .launch:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .launch:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .game-header {
    display: flex;
    justify-content: space-between;
    gap: 2rem;
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .default-header {
    padding: 1rem 1.25rem;
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: var(--bg-secondary);
  }

  .default-header h1 {
    margin: 0 0 0.4rem;
  }

  .default-note {
    margin: 0;
    font-size: 0.85rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  h1 {
    font-size: 1.6rem;
    margin-bottom: 0;
    color: var(--text-primary);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  h2 {
    font-size: 0.85rem;
    color: var(--accent-secondary);
    margin-bottom: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .info {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.75rem 1rem;
    background-color: var(--bg-secondary);
  }

  .row {
    display: flex;
    margin-bottom: 0.4rem;
  }

  .row:last-child {
    margin-bottom: 0;
  }

  .label {
    width: 100px;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: 0.7rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .value {
    color: var(--text-primary);
    word-break: break-all;
    font-size: 0.85rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .section {
    margin-bottom: 1.5rem;
  }

  .profile-grid .section {
    margin-bottom: 0;
  }

  .section.boxed h2 {
    margin-bottom: 0.5rem;
  }

  .section.boxed:last-child {
    margin-bottom: 0;
  }

  .dll-table {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.5rem 0.75rem;
    background-color: var(--bg-secondary);
  }

  .dll-row {
    display: grid;
    grid-template-columns: repeat(4, minmax(80px, 1fr));
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

  .profile-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 1rem;
    margin-top: 0.5rem;
  }

  @media (min-width: 80ch) {
    .profile-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  .section.boxed {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.75rem;
    background-color: var(--bg-secondary);
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .field {
    margin-bottom: 0;
  }

  .field label {
    display: block;
    color: var(--text-dim);
    margin-bottom: 0.25rem;
    font-size: 0.8rem;
    letter-spacing: 0.02em;
    text-transform: uppercase;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .field.checkbox {
    display: grid;
    grid-template-columns: auto 1fr;
    column-gap: 0.5rem;
    row-gap: 0.2rem;
    align-items: start;
  }

  .field.checkbox label {
    margin-bottom: 0;
    text-transform: none;
    letter-spacing: 0.02em;
  }

  .field.checkbox input {
    width: 18px;
    height: 18px;
    accent-color: var(--accent-primary);
    margin-top: 0.15rem;
  }

  .field.checkbox label span {
    text-transform: none;
  }

  .save {
    width: 100%;
    padding: 0.75rem;
    border: none;
    border-radius: 0;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    cursor: pointer;
    font-size: 0.9rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .save:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .save:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .message {
    margin-top: 0.75rem;
    padding: 0.5rem;
    border-radius: 0;
    text-align: center;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .message[data-type='success'] {
    color: var(--success);
    border-color: rgba(118, 185, 0, 0.4);
  }

  .message[data-type='error'] {
    color: var(--error);
    border-color: rgba(255, 107, 107, 0.4);
  }

  .hint {
    display: block;
    font-size: 0.72rem;
    color: var(--text-dim);
    margin-top: 0.2rem;
    line-height: 1.3;
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .field.checkbox .hint {
    grid-column: 2;
  }

  .actions {
    margin-top: 0.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-default);
  }

  @media (max-width: 1100px) {
    .game-header {
      flex-direction: column;
    }
  }
</style>
