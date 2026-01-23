<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetConfig, SaveConfig, GetVersion } from '../wailsjs/go/gui/App'
  import { Quit } from '../wailsjs/runtime/runtime'
  import Header from './lib/Header.svelte'
  import GameList from './lib/GameList.svelte'
  import GameDetail from './lib/GameDetail.svelte'

  const wailsBindings = { GetConfig, SaveConfig, GetVersion }

  let selectedGame = null
  let selectedProfileMode = 'game'
  let gameListComponent
  let gameDetailComponent
  let theme = 'dark'
  let showOptions = false
  let showHelp = false
  let focusPane = 'list'
  let config = null
  let version = ''
  let configMessage = ''
  let configMessageType = 'info'
  let configMessageTimer

  const optionSections = [
    {
      title: 'Display',
      options: [
        {
          key: 'theme',
          label: 'Theme',
          description: 'Match the system theme or force dark mode.',
          type: 'select',
          choices: ['default', 'dark']
        },
        {
          key: 'showHints',
          label: 'Show hints',
          description: 'Show keyboard hints in the footer and dialogs.',
          type: 'toggle'
        },
        {
          key: 'compactMode',
          label: 'Compact mode',
          description: 'Use tighter spacing in lists and panels.',
          type: 'toggle'
        }
      ]
    },
    {
      title: 'Startup',
      options: [
        {
          key: 'rescanOnStartup',
          label: 'Re-scan on startup',
          description: 'Scan for games whenever Spela launches.',
          type: 'toggle'
        },
        {
          key: 'autoUpdateDLLs',
          label: 'Auto-update DLLs',
          description: 'Update DLLs automatically when the app starts.',
          type: 'toggle'
        },
        {
          key: 'checkUpdates',
          label: 'Check for updates',
          description: 'Look for new Spela releases at startup.',
          type: 'toggle'
        }
      ]
    },
    {
      title: 'Paths',
      options: [
        {
          key: 'steamPath',
          label: 'Steam path',
          description: 'Custom Steam installation path.',
          type: 'path'
        },
        {
          key: 'dllCachePath',
          label: 'DLL cache path',
          description: 'Override where downloaded DLLs are stored.',
          type: 'path'
        },
        {
          key: 'backupPath',
          label: 'Backup path',
          description: 'Location for DLL and save backups.',
          type: 'path'
        }
      ]
    },
    {
      title: 'System',
      options: [
        {
          key: 'logLevel',
          label: 'Log level',
          description: 'Control logging verbosity for troubleshooting.',
          type: 'select',
          choices: ['debug', 'info', 'warn', 'error']
        },
        {
          key: 'confirmDestructive',
          label: 'Confirm destructive',
          description: 'Ask before destructive actions like restores.',
          type: 'toggle'
        }
      ]
    },
    {
      title: 'DLL management',
      options: [
        {
          key: 'autoRefreshManifest',
          label: 'Auto-refresh manifest',
          description: 'Refresh the DLL manifest automatically.',
          type: 'toggle'
        },
        {
          key: 'manifestRefreshHours',
          label: 'Refresh interval',
          description: 'How often to refresh the manifest (hours).',
          type: 'select',
          choices: ['1', '6', '12', '24', '48', '168']
        },
        {
          key: 'preferredDLLSource',
          label: 'DLL source',
          description: 'Preferred source for DLL downloads.',
          type: 'select',
          choices: ['techpowerup', 'github']
        }
      ]
    }
  ]

  let optionsState = {
    theme: 'dark',
    showHints: true,
    compactMode: false,
    logLevel: 'info',
    steamPath: '',
    dllCachePath: '',
    backupPath: '',
    confirmDestructive: true,
    rescanOnStartup: false,
    autoUpdateDLLs: false,
    checkUpdates: false,
    autoRefreshManifest: true,
    manifestRefreshHours: '24',
    preferredDLLSource: 'techpowerup'
  }

  onMount(() => {
    loadConfig()
    loadVersion()
    window.addEventListener('keydown', handleKeydown)
  })

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown)
  })

  function selectGame(game) {
    selectedProfileMode = 'game'
    selectedGame = game
  }

  function selectDefaultProfile() {
    selectedProfileMode = 'default'
    selectedGame = null
  }

  function toggleHelp() {
    showHelp = !showHelp
  }

  function setFocusPane(nextPane) {
    if (nextPane === 'detail' && !gameDetailComponent) {
      focusPane = 'list'
      gameListComponent?.focusSearch?.()
      return
    }
    focusPane = nextPane
    if (focusPane === 'list') {
      gameListComponent?.focusSearch?.()
    } else {
      gameDetailComponent?.focusPrimary?.()
    }
  }

  function isEditableTarget(target) {
    if (!target || !(target instanceof HTMLElement)) {
      return false
    }
    const tagName = target.tagName.toLowerCase()
    return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable
  }

  function handleKeydown(event) {
    if (event.key === 'Tab' && !showOptions && !showHelp) {
      event.preventDefault()
      const nextPane = focusPane === 'list' ? 'detail' : 'list'
      setFocusPane(nextPane)
      return
    }

    if (isEditableTarget(event.target)) {
      return
    }

    if (event.key === 'q' || event.key === 'Q') {
      event.preventDefault()
      Quit()
      return
    }

    if (event.key === '?') {
      event.preventDefault()
      toggleHelp()
      return
    }

    if (event.key === 'Escape' && showHelp) {
      event.preventDefault()
      showHelp = false
      return
    }
  }

  async function handleGameUpdate(event) {
    const updated = event.detail
    if (updated && selectedProfileMode === 'game') {
      selectedGame = updated
      if (gameListComponent?.refreshGames) {
        await gameListComponent.refreshGames()
      }
    }
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
        logLevel: loaded.logLevel || 'info',
        steamPath: loaded.steamPath || '',
        dllCachePath: loaded.dllCachePath || '',
        backupPath: loaded.backupPath || '',
        confirmDestructive: loaded.confirmDestructive ?? true,
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

  async function loadVersion() {
    try {
      version = await wailsBindings.GetVersion()
    } catch (error) {
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
      logLevel: optionsState.logLevel,
      steamPath: optionsState.steamPath,
      dllCachePath: optionsState.dllCachePath,
      backupPath: optionsState.backupPath,
      confirmDestructive: optionsState.confirmDestructive,
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
  <Header on:options={toggleOptions} />

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
              <div class="options-label">
                <div class="options-label-title">{option.label}</div>
                {#if option.description}
                  <div class="options-description">{option.description}</div>
                {/if}
              </div>
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
                {:else if option.type === 'path'}
                  <input
                    type="text"
                    class="path-input"
                    placeholder="(default)"
                    value={optionsState[option.key]}
                    on:input={event => updateOption(option.key, event.currentTarget.value)}
                  />
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

  {#if showHelp}
    <button
      type="button"
      class="options-overlay"
      aria-label="Close help"
      on:click={toggleHelp}
    ></button>
    <div class="help-panel" role="dialog" aria-modal="true" aria-label="Help">
      <div class="options-header">
        <div class="options-title">Help</div>
        <button class="options-close" on:click={toggleHelp}>Close</button>
      </div>
      <div class="help-section">
        <div class="help-title">Navigation</div>
        <div class="help-rows">
          <div class="help-row"><span class="help-key">Tab</span><span>Switch between list and detail</span></div>
          <div class="help-row"><span class="help-key">?</span><span>Toggle this help</span></div>
          <div class="help-row"><span class="help-key">Q</span><span>Quit</span></div>
        </div>
      </div>
      <div class="help-section">
        <div class="help-title">List</div>
        <div class="help-rows">
          <div class="help-row"><span class="help-key">/</span><span>Search games</span></div>
          <div class="help-row"><span class="help-key">D</span><span>Toggle DLL filter</span></div>
          <div class="help-row"><span class="help-key">P</span><span>Toggle profile filter</span></div>
          <div class="help-row"><span class="help-key">S</span><span>Cycle sort mode</span></div>
          <div class="help-row"><span class="help-key">R</span><span>Rescan games</span></div>
        </div>
      </div>
      <div class="help-section">
        <div class="help-title">Detail</div>
        <div class="help-rows">
          <div class="help-row"><span class="help-key">L</span><span>Launch game</span></div>
          <div class="help-row"><span class="help-key">I</span><span>Install DLL</span></div>
          <div class="help-row"><span class="help-key">U</span><span>Update DLLs</span></div>
          <div class="help-row"><span class="help-key">R</span><span>Restore DLLs</span></div>
        </div>
      </div>
    </div>
  {/if}

  <div class="app-shell">
    <aside class="sidebar">
      <GameList
        bind:this={gameListComponent}
        selectedGame={selectedGame}
        defaultProfileSelected={selectedProfileMode === 'default'}
        on:select={e => selectGame(e.detail)}
        on:selectDefaultProfile={selectDefaultProfile}
      />
    </aside>
    <section class="content">
      {#if selectedProfileMode === 'default'}
        <GameDetail
          bind:this={gameDetailComponent}
          profileMode="default"
          on:gameUpdate={handleGameUpdate}
        />
      {:else if selectedGame}
        <GameDetail
          bind:this={gameDetailComponent}
          game={selectedGame}
          profileMode="game"
          on:gameUpdate={handleGameUpdate}
        />
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
    border-radius: 0;
    margin-bottom: 0.6rem;
    font-size: 0.75rem;
    color: var(--text-primary);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
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


  .options-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    border: none;
    padding: 0;
    z-index: 3000;
  }


  .options-panel {
    position: fixed;
    top: 6rem;
    right: 2rem;
    width: min(460px, 92vw);
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 1rem;
    z-index: 3001;
    text-transform: none;
    font-size: 0.85rem;
  }

  .help-panel {
    position: fixed;
    top: 8rem;
    left: 50%;
    transform: translateX(-50%);
    width: min(560px, 92vw);
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 1rem 1.2rem;
    z-index: 3001;
  }

  .help-section {
    padding: 0.75rem 0;
    border-top: 1px solid var(--border-default);
  }

  .help-section:first-of-type {
    border-top: none;
  }

  .help-title {
    font-size: 0.75rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent-secondary);
    margin-bottom: 0.5rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .help-rows {
    display: grid;
    gap: 0.35rem;
  }

  .help-row {
    display: grid;
    grid-template-columns: 4rem 1fr;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .help-key {
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
    font-weight: 600;
    color: var(--text-primary);
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
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .options-close {
    border: none;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.7rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
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
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .options-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1.2rem;
    padding: 0.45rem 0;
  }

  .options-label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.7rem;
    color: var(--text-dim);
    text-transform: none;
    letter-spacing: 0.01em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .options-label-title {
    color: var(--text-primary);
    font-weight: 600;
  }

  .options-description {
    font-size: 0.65rem;
    line-height: 1.3;
    max-width: 220px;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .options-control {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .path-input {
    min-width: 220px;
    padding: 0.3rem 0.6rem;
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    font-size: 0.75rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .path-input:focus {
    outline: none;
    border-color: var(--border-focus);
  }

  .toggle {
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: transparent;
    color: var(--text-dim);
    padding: 0.25rem 0.6rem;
    font-size: 0.7rem;
    cursor: pointer;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
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
    border-radius: 0;
    background-color: transparent;
    color: var(--text-dim);
    padding: 0.2rem 0.5rem;
    font-size: 0.7rem;
    cursor: pointer;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
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
    gap: 0.75rem;
    padding: 0 1.5rem 1rem;
    min-height: 0;
    background-color: var(--bg-primary);
  }

  .sidebar,
  .content {
    position: relative;
  }

  .sidebar,
  .content {
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: var(--bg-secondary);
    overflow: visible;
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
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
    text-transform: none;
    letter-spacing: 0.04em;
    background-color: var(--bg-secondary);
  }

  .footer-message {
    color: var(--accent-secondary);
    text-transform: none;
    letter-spacing: 0.01em;
    font-size: 0.7rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  @media (max-width: 1100px) {
    .content {
      padding: 1rem;
    }
  }

  @media (max-width: 720px) {
    .app-shell {
      padding: 0 1rem 0.75rem;
      gap: 0.75rem;
    }

    .sidebar,
    .content {
      border-radius: 0;
    }

    .footer {
      padding: 0.5rem 1rem 0.75rem;
    }
  }
</style>
