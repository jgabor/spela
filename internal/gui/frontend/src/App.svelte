<script>
  import { onMount, onDestroy } from 'svelte'
  import Header from './lib/Header.svelte'
  import GameDetail from './lib/GameDetail.svelte'
  import PrimaryNav from './lib/PrimaryNav.svelte'
  import ContextNav from './lib/ContextNav.svelte'
  import SettingsPane from './lib/SettingsPane.svelte'
  import MonitorPane from './lib/Monitor.svelte'
  import DLLCatalogPane from './lib/DLLCatalogPane.svelte'
  import GameOverview from './lib/GameOverview.svelte'
  import { desktopCommands } from './lib/desktop'
  import {
    defaultNavState,
    Destination,
    destinationFromHotkey,
    Aspect,
    breadcrumb,
    selectDestination as applyDestination,
    selectScope,
    selectAspect,
    selectSubsystem,
    selectDLLSection,
    selectMonitorSection,
    selectSettingsSection,
    initialNavState
  } from './lib/navState.js'

  export let desktop = desktopCommands

  let selectedGame = null
  let nav = initialNavState()
  let gameDetailComponent
  let contextNavComponent
  let theme = 'dark'
  let showHelp = false
  let config = null
  let version = ''
  let configMessage = ''
  let configMessageType = 'info'
  let configMessageTimer

  let optionSections = []

  let optionsState = {}

  function defaultOptionValue(option) {
    if (option.type === 'toggle') {
      return false
    }
    if (option.choices?.length) {
      return option.choices[0]
    }
    return ''
  }

  function buildOptionsState(sections, loaded = {}) {
    const state = {}
    for (const section of sections) {
      for (const option of section.options || []) {
        const loadedVal = loaded[option.key]
        if (loadedVal !== undefined && loadedVal !== null && loadedVal !== '') {
          state[option.key] = option.type === 'toggle' ? !!loadedVal : String(loadedVal)
        } else {
          state[option.key] = defaultOptionValue(option)
        }
      }
    }
    return state
  }

  onMount(async () => {
    nav = await defaultNavState()
    await loadSettingsCatalog()
    loadConfig()
    loadVersion()
    window.addEventListener('keydown', handleKeydown)
  })

  async function loadSettingsCatalog() {
    try {
      if (desktop.GetSettingsCatalog) {
        optionSections = await desktop.GetSettingsCatalog()
      }
    } catch (error) {
      optionSections = []
    }
  }

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown)
  })

  async function selectGame(game) {
    selectedGame = game
    nav = await selectScope(nav, false, game?.name ?? '')
    if (nav.aspect !== Aspect.Profile && nav.aspect !== Aspect.DLLs) {
      nav = await selectAspect(nav, Aspect.Overview)
    }
  }

  async function selectDefaultProfile() {
    selectedGame = null
    nav = await selectScope(nav, true)
  }

  async function selectDestination(event) {
    nav = await applyDestination(nav, event.detail.destination)
    if (event.detail.destination === Destination.Settings) {
      loadConfig()
    }
  }

  async function handleSelectAspect(event) {
    nav = await selectAspect(nav, event.detail.aspect)
  }

  async function handleSelectSubsystem(event) {
    nav = await selectSubsystem(nav, event.detail.subsystem)
  }

  async function handleDLLSection(event) {
    nav = await selectDLLSection(nav, event.detail.section)
  }

  async function handleMonitorSection(event) {
    nav = await selectMonitorSection(nav, event.detail.section)
  }

  async function handleSettingsSection(event) {
    nav = await selectSettingsSection(nav, event.detail.section)
  }

  $: crumb = breadcrumb(nav).join(' › ')
  $: activeSettingsSection = optionSections.find(section => section.id === nav.settingsSection) ?? optionSections[nav.settingsSection]
  $: gameDetailProps = (() => {
    if (nav.destination !== Destination.Library) {
      return null
    }
    if (nav.scopeGlobal && nav.aspect === Aspect.Profile) {
      return { profileMode: 'default', profileSubsystem: nav.subsystem, game: null, aspect: 'profile' }
    }
    if (!selectedGame || nav.aspect === Aspect.Overview) {
      return null
    }
    if (nav.aspect === Aspect.DLLs) {
      return { profileMode: 'game', game: selectedGame, aspect: 'dlls', profileSubsystem: 0 }
    }
    return { profileMode: 'game', game: selectedGame, aspect: 'profile', profileSubsystem: nav.subsystem }
  })()

  function toggleHelp() {
    showHelp = !showHelp
  }

  function isEditableTarget(target) {
    if (!target || !(target instanceof HTMLElement)) {
      return false
    }
    const tagName = target.tagName.toLowerCase()
    return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable
  }

  function handleKeydown(event) {
    if (isEditableTarget(event.target)) {
      return
    }

    if (!showHelp) {
      const dest = destinationFromHotkey(event.key)
      if (dest !== null) {
        event.preventDefault()
        selectDestination({ detail: { destination: dest } })
        return
      }
    }

    if (event.key === 'Tab' && !showHelp && nav.destination === Destination.Library) {
      event.preventDefault()
      contextNavComponent?.focusSearch?.()
      return
    }

    if (event.key === 'q' || event.key === 'Q') {
      event.preventDefault()
      desktop.Quit()
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
    if (updated && !nav.scopeGlobal) {
      selectedGame = updated
      await contextNavComponent?.gameListComponent?.refreshGames?.()
    }
  }

  async function openSettings() {
    nav = await applyDestination(nav, Destination.Settings)
    loadConfig()
  }

  async function loadConfig() {
    try {
      const loaded = await desktop.GetConfig()
      config = loaded
      optionsState = buildOptionsState(optionSections, loaded)
      theme = optionsState.theme || loaded.theme || 'dark'
      document.documentElement.setAttribute('data-theme', theme)
      resetOptionsMessage()
    } catch (error) {
      setConfigMessage('Failed to load config', 'error')
    }
  }

  async function loadVersion() {
    try {
      version = await desktop.GetVersion()
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
    try {
      await desktop.SaveConfigOption(key, String(value))
      config = { ...config, [key]: value }
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
  <Header {desktop} on:options={openSettings} />

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
          <div class="help-row"><span class="help-key">1-4</span><span>Primary destinations (Library, DLL Catalog, Monitor, Settings)</span></div>
          <div class="help-row"><span class="help-key">Tab</span><span>Focus game search (Library)</span></div>
          <div class="help-row"><span class="help-key">?</span><span>Toggle this help</span></div>
          <div class="help-row"><span class="help-key">Q</span><span>Quit</span></div>
        </div>
      </div>
      <div class="help-section">
        <div class="help-title">Library scope</div>
        <div class="help-rows">
          <div class="help-row"><span class="help-key">/</span><span>Search games</span></div>
          <div class="help-row"><span class="help-key">D</span><span>Toggle DLL filter</span></div>
          <div class="help-row"><span class="help-key">P</span><span>Toggle profile filter</span></div>
          <div class="help-row"><span class="help-key">S</span><span>Cycle sort mode</span></div>
          <div class="help-row"><span class="help-key">R</span><span>Rescan games</span></div>
        </div>
      </div>
      <div class="help-section">
        <div class="help-title">Launch</div>
        <div class="help-rows">
          <div class="help-row"><span class="help-key">Steam</span><span>Set launch options to <code>spela %command%</code></span></div>
        </div>
      </div>
    </div>
  {/if}

  <div class="app-shell">
    <PrimaryNav active={nav.destination} on:select={selectDestination} />
    <ContextNav
      bind:this={contextNavComponent}
      {desktop}
      destination={nav.destination}
      {selectedGame}
      scopeGlobal={nav.scopeGlobal}
      aspect={nav.aspect}
      subsystem={nav.subsystem}
      dllSection={nav.dllSection}
      monitorSection={nav.monitorSection}
      settingsSection={nav.settingsSection}
      on:selectGame={e => selectGame(e.detail)}
      on:selectGlobal={selectDefaultProfile}
      on:selectAspect={handleSelectAspect}
      on:selectSubsystem={handleSelectSubsystem}
      on:dllSection={handleDLLSection}
      on:monitorSection={handleMonitorSection}
      on:settingsSection={handleSettingsSection}
    />
    <section class="content">
      {#if nav.destination === Destination.Settings}
        <SettingsPane
          section={activeSettingsSection}
          {optionsState}
          {configMessage}
          configMessageType={configMessageType}
          onOptionChange={updateOption}
        />
      {:else if nav.destination === Destination.Monitor}
        <MonitorPane {desktop} section={nav.monitorSection} />
      {:else if nav.destination === Destination.DLLCatalog}
        <DLLCatalogPane {desktop} section={nav.dllSection} />
      {:else if gameDetailProps}
        <GameDetail
          bind:this={gameDetailComponent}
          {desktop}
          {...gameDetailProps}
          on:gameUpdate={handleGameUpdate}
        />
      {:else if selectedGame && nav.aspect === Aspect.Overview}
        <GameOverview game={selectedGame} />
      {:else}
        <div class="empty-state">Select a scope from the list</div>
      {/if}
    </section>
  </div>

  <footer class="footer">
    <div class="footer-message">Spela › {crumb}</div>
    <div class="footer-hints">1-4: navigate • tab: search • ?: help • q: quit{#if version} • v{version}{/if}</div>
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


  .options-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    border: none;
    padding: 0;
    z-index: 3000;
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

  .app-shell {
    flex: 1;
    display: grid;
    grid-template-columns: minmax(9rem, 11rem) minmax(14rem, 18rem) minmax(0, 1fr);
    gap: 0;
    padding: 0 1.5rem 1rem;
    min-height: 0;
    background-color: var(--bg-primary);
    border: 1px solid var(--border-default);
    margin: 0 1.5rem;
  }

  .app-shell :global(.primary-nav),
  .app-shell :global(.context-nav) {
    min-height: 0;
  }

  .content {
    position: relative;
    border-left: 1px solid var(--border-default);
    background-color: var(--bg-secondary);
    display: flex;
    flex-direction: column;
    min-height: 0;
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

  @media (max-width: 900px) {
    .app-shell {
      grid-template-columns: minmax(8rem, 10rem) minmax(12rem, 16rem) minmax(0, 1fr);
      margin: 0 1rem;
      padding: 0 1rem 0.75rem;
    }

    .footer {
      padding: 0.5rem 1rem 0.75rem;
    }
  }
</style>
