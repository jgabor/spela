<script>
  import { onMount } from 'svelte'
  import { GetGPUInfo, GetCPUInfo, GetConfig, SaveConfig, GetVersion, GetLogo } from '../wailsjs/go/gui/App'
  import GameList from './lib/GameList.svelte'
  import GameDetail from './lib/GameDetail.svelte'

  const wailsBindings = { GetConfig, SaveConfig, GetVersion, GetLogo }
  const metricsBindings = { GetGPUInfo, GetCPUInfo }

  let selectedGame = null
  let gpu = null
  let cpu = null
  let refreshTimer
  let theme = 'dark'
  let showOptions = false
  let config = null
  let logoSource = null
  let version = ''
  let configMessage = ''
  let configMessageType = 'info'
  let configMessageTimer
  const logoText = 'Spela'

  const optionSections = [
    {
      title: 'Display',
      options: [
        { key: 'theme', label: 'Theme', type: 'select', choices: ['default', 'dark'] },
        { key: 'showHints', label: 'Show hints', type: 'toggle' },
        { key: 'compactMode', label: 'Compact mode', type: 'toggle' }
      ]
    },
    {
      title: 'Startup',
      options: [
        { key: 'rescanOnStartup', label: 'Re-scan on startup', type: 'toggle' },
        { key: 'autoUpdateDLLs', label: 'Auto-update DLLs', type: 'toggle' },
        { key: 'checkUpdates', label: 'Check for updates', type: 'toggle' }
      ]
    },
    {
      title: 'DLL management',
      options: [
        { key: 'autoRefreshManifest', label: 'Auto-refresh manifest', type: 'toggle' },
        { key: 'manifestRefreshHours', label: 'Refresh interval', type: 'select', choices: ['1', '6', '12', '24', '48', '168'] },
        { key: 'preferredDLLSource', label: 'DLL source', type: 'select', choices: ['techpowerup', 'github'] }
      ]
    }
  ]

  let optionsState = {
    theme: 'dark',
    showHints: true,
    compactMode: false,
    rescanOnStartup: false,
    autoUpdateDLLs: false,
    checkUpdates: false,
    autoRefreshManifest: true,
    manifestRefreshHours: '24',
    preferredDLLSource: 'techpowerup'
  }

  onMount(() => {
    loadConfig()
    loadAssets()
    refreshMetrics()
    refreshTimer = setInterval(refreshMetrics, 2000)
    return () => {
      clearInterval(refreshTimer)
    }
  })

  async function refreshMetrics() {
    try {
      gpu = await metricsBindings.GetGPUInfo()
      cpu = await metricsBindings.GetCPUInfo()
    } catch (error) {
      gpu = null
      cpu = null
    }
  }

  function selectGame(game) {
    selectedGame = game
  }

  function toggleOptions() {
    showOptions = !showOptions
    if (showOptions) {
      resetOptionsMessage()
    }
  }

  async function loadConfig() {
    try {
      const loaded = await wailsBindings.GetConfig()
      config = loaded
      optionsState = {
        theme: loaded.theme || 'default',
        showHints: loaded.showHints,
        compactMode: loaded.compactMode,
        rescanOnStartup: loaded.rescanOnStartup,
        autoUpdateDLLs: loaded.autoUpdateDLLs,
        checkUpdates: loaded.checkUpdates,
        autoRefreshManifest: loaded.autoRefreshManifest,
        manifestRefreshHours: String(loaded.manifestRefreshHours || 24),
        preferredDLLSource: loaded.preferredDLLSource || 'techpowerup'
      }
      theme = optionsState.theme
      document.documentElement.setAttribute('data-theme', theme)
      resetOptionsMessage()
    } catch (error) {
      setConfigMessage('Failed to load config', 'error')
    }
  }

  async function loadAssets() {
    try {
      logoSource = await wailsBindings.GetLogo()
      version = await wailsBindings.GetVersion()
    } catch (error) {
      logoSource = null
      version = ''
    }
  }

  function clearConfigMessageAfter(delay) {
    if (configMessageTimer) {
      clearTimeout(configMessageTimer)
    }
    configMessageTimer = setTimeout(() => {
      configMessage = ''
      configMessageTimer = null
    }, delay)
  }

  function setConfigMessage(message, type) {
    configMessage = message
    configMessageType = type
    clearConfigMessageAfter(3000)
  }

  async function updateOption(key, value) {
    optionsState = { ...optionsState, [key]: value }
    if (key === 'theme') {
      theme = value
      document.documentElement.setAttribute('data-theme', value)
    }
    if (!config) {
      return
    }
    const updated = {
      ...config,
      theme: optionsState.theme,
      showHints: optionsState.showHints,
      compactMode: optionsState.compactMode,
      rescanOnStartup: optionsState.rescanOnStartup,
      autoUpdateDLLs: optionsState.autoUpdateDLLs,
      checkUpdates: optionsState.checkUpdates,
      autoRefreshManifest: optionsState.autoRefreshManifest,
      manifestRefreshHours: Number(optionsState.manifestRefreshHours),
      preferredDLLSource: optionsState.preferredDLLSource
    }
    try {
      await wailsBindings.SaveConfig(updated)
      config = updated
      setConfigMessage('Options saved', 'success')
    } catch (error) {
      setConfigMessage('Failed to save options', 'error')
    }
  }

  function resetOptionsMessage() {
    configMessage = ''
  }
</script>

<main>
  <header class="app-header">
    <div class="logo">
      {#if logoSource}
        <img src={logoSource} alt="Spela" class="logo-image" />
      {:else}
        <div class="logo-text">{logoText}</div>
      {/if}
    </div>
    <div class="header-right">
      <div class="metrics">
        <div class="metric-line">
          <span class="metric-label">GPU:</span>
          <span class="metric-value">
            {#if gpu}
              {gpu.temperature}°C {gpu.utilization}% {gpu.powerDraw.toFixed(0)}W
            {:else}
              N/A
            {/if}
          </span>
        </div>
        <div class="metric-line">
          <span class="metric-label">VRAM:</span>
          <span class="metric-value">
            {#if gpu}
              {(gpu.memoryUsed / 1024).toFixed(1)}/{(gpu.memoryTotal / 1024).toFixed(1)} GB
            {:else}
              N/A
            {/if}
          </span>
        </div>
        <div class="metric-line">
          <span class="metric-label">CPU:</span>
          <span class="metric-value">
            {#if cpu}
              {cpu.utilizationPercent.toFixed(0)}% {cpu.averageFrequency}MHz
            {:else}
              N/A
            {/if}
          </span>
        </div>
        <div class="metric-line">
          <span class="metric-label">RAM:</span>
          <span class="metric-value">
            {#if cpu}
              {(cpu.memoryUsedMegabytes / 1024).toFixed(1)}/{(cpu.memoryTotalMegabytes / 1024).toFixed(1)} GB
            {:else}
              N/A
            {/if}
          </span>
        </div>
      </div>
      <button class="options-button" on:click={toggleOptions}>Options</button>
    </div>
  </header>

  {#if showOptions}
    <button
      type="button"
      class="options-overlay"
      aria-label="Close options"
      on:click={toggleOptions}
    ></button>
    <div class="options-panel" role="dialog" aria-modal="true" aria-label="Options">
      <div class="options-header">
        <div class="options-title">Options</div>
        <button class="options-close" on:click={toggleOptions}>Close</button>
      </div>
      {#if configMessage}
        <div class="options-message" data-type={configMessageType}>{configMessage}</div>
      {/if}
      {#each optionSections as section}
        <div class="options-section">
          <div class="options-section-title">{section.title}</div>
          {#each section.options as option}
            <div class="options-row">
              <div class="options-label">{option.label}</div>
              <div class="options-control">
                {#if option.type === 'toggle'}
                  <button
                    class="toggle"
                    class:active={optionsState[option.key]}
                    on:click={() => updateOption(option.key, !optionsState[option.key])}
                  >
                    {optionsState[option.key] ? 'On' : 'Off'}
                  </button>
                {:else if option.type === 'select'}
                  <div class="select-list">
                    {#each option.choices as choice}
                      <button
                        class="select-option"
                        class:active={optionsState[option.key] === choice}
                        on:click={() => updateOption(option.key, choice)}
                      >
                        {choice}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/each}
      <div class="options-footer">
        Changes are saved to config.yaml.
      </div>
    </div>
  {/if}

  <div class="app-shell">
    <aside class="sidebar">
      <GameList on:select={e => selectGame(e.detail)} selectedGame={selectedGame} />
    </aside>
    <section class="content">
      {#if selectedGame}
        <GameDetail game={selectedGame} />
      {:else}
        <div class="empty-state">Select a game from the list</div>
      {/if}
    </section>
  </div>

  <footer class="footer">
    <div class="footer-message">NVIDIA DLSS super resolution and frame generation settings</div>
    <div class="footer-hints">tab: switch • ?: help • q: quit{#if version} • v{version}{/if}</div>
  </footer>
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    position: relative;
  }

  .app-header {
    display: flex;
    justify-content: space-between;
    padding: 0.75rem 1.5rem 1rem;
    border-bottom: 1px solid var(--border-default);
    background-color: var(--bg-primary);
  }

  .logo {
    display: flex;
    align-items: center;
    min-height: 44px;
  }

  .logo img {
    image-rendering: auto;
  }

  .logo-text {
    font-size: 2rem;
    font-weight: 700;
    color: var(--accent-primary);
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .logo-image {
    height: 42px;
    width: auto;
    display: block;
  }

    .header-right {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
  }

  .metrics {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.75rem;
    align-items: flex-end;
    letter-spacing: 0.02em;
  }

  .metrics .metric-label {
    text-transform: uppercase;
  }

  .metrics .metric-value {
    text-transform: none;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .metric-value {
    font-weight: 600;
  }

  .metric-line {
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .options-panel button {
    font-family: inherit;
    text-transform: none;
  }

  .options-panel,
  .options-panel * {
    text-transform: none;
  }

  .options-panel .options-title,
  .options-panel .options-section-title {
    text-transform: uppercase;
  }

  .options-message {
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    margin-bottom: 0.6rem;
    font-size: 0.75rem;
    color: var(--text-primary);
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .options-message[data-type='success'] {
    border-color: rgba(118, 185, 0, 0.4);
    color: var(--success);
  }

  .options-message[data-type='error'] {
    border-color: rgba(255, 107, 107, 0.4);
    color: var(--error);
  }

  .options-footer {
    margin-top: 1rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border-default);
    font-size: 0.7rem;
    color: var(--text-dim);
  }

  .metric-line {
    display: flex;
    gap: 0.5rem;
  }

  .metric-label {
    color: var(--text-dim);
    text-transform: uppercase;
  }

  .metric-value {
    color: var(--text-primary);
  }

  .options-button {
    padding: 0.45rem 0.9rem;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background-color: transparent;
    color: var(--text-primary);
    font-size: 0.75rem;
    cursor: pointer;
    align-self: flex-start;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .options-button:hover {
    border-color: var(--border-focus);
    color: var(--accent-primary);
  }

  .options-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    border: none;
    padding: 0;
    z-index: 10;
  }


  .options-panel {
    position: fixed;
    top: 6rem;
    right: 2rem;
    width: min(460px, 92vw);
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    border-radius: 8px;
    padding: 1rem;
    z-index: 11;
    text-transform: none;
    font-size: 0.85rem;
  }

  .options-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .options-title {
    font-size: 0.9rem;
    letter-spacing: 0.08em;
  }

  .options-close {
    border: none;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.7rem;
  }

  .options-close:hover {
    color: var(--text-primary);
  }

  .options-section {
    border-top: 1px solid var(--border-default);
    padding-top: 0.75rem;
    margin-top: 0.75rem;
  }

  .options-section:first-of-type {
    border-top: none;
    padding-top: 0;
    margin-top: 0;
  }

  .options-section-title {
    font-size: 0.7rem;
    color: var(--accent-secondary);
    letter-spacing: 0.08em;
    margin-bottom: 0.5rem;
    text-transform: uppercase;
  }

  .options-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: 0.45rem 0;
  }

  .options-label {
    font-size: 0.7rem;
    color: var(--text-dim);
    text-transform: none;
    letter-spacing: 0.01em;
  }

  .options-control {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .toggle {
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background-color: transparent;
    color: var(--text-dim);
    padding: 0.25rem 0.6rem;
    font-size: 0.7rem;
    cursor: pointer;
  }

  .toggle.active {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    border-color: var(--accent-primary);
  }

  .select-list {
    display: flex;
    gap: 0.4rem;
  }

  .select-option {
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background-color: transparent;
    color: var(--text-dim);
    padding: 0.2rem 0.5rem;
    font-size: 0.7rem;
    cursor: pointer;
  }

  .select-option.active {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    border-color: var(--accent-primary);
  }

  @media (max-width: 720px) {
    .options-panel {
      right: 1rem;
      left: 1rem;
      top: 5rem;
    }
  }

  .app-shell {
    flex: 1;
    display: grid;
    grid-template-columns: clamp(25ch, 30%, 50ch) minmax(0, 1fr);
    gap: 1rem;
    padding: 1rem 1.5rem;
    min-height: 0;
    background-color: var(--bg-primary);
  }

  .sidebar,
  .content {
    position: relative;
    z-index: 1;
  }

  .sidebar,
  .content {
    border: 1px solid var(--border-default);
    border-radius: var(--border-radius);
    background-color: var(--bg-primary);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .sidebar {
    padding: 0.75rem;
  }


  .content {
    padding: 1rem 1.25rem;
    overflow-y: auto;
  }

  .empty-state {
    color: var(--text-dim);
    text-align: center;
    margin-top: 3rem;
  }

  .footer {
    border-top: 1px solid var(--border-default);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.5rem 1.5rem 0.75rem;
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.02em;
  }

  .footer-message {
    color: var(--accent-secondary);
    text-transform: none;
    letter-spacing: 0.01em;
    font-size: 0.7rem;
  }

  @media (max-width: 1100px) {
    .content {
      padding: 1rem;
    }

    .header-right {
      flex-direction: column;
      align-items: flex-end;
    }
  }

  @media (max-width: 720px) {
    .app-header {
      flex-direction: column;
      gap: 1rem;
    }

    .header-right {
      width: 100%;
      align-items: flex-start;
    }

    .metrics {
      align-items: flex-start;
    }

    .app-shell {
      padding: 0.75rem 1rem;
      gap: 0.75rem;
    }

    .sidebar,
    .content {
      border-radius: var(--border-radius-small);
    }

    .footer {
      padding: 0.5rem 1rem 0.75rem;
    }
  }
</style>
