<script>
  export let game = null
</script>

{#if !game}
  <p class="empty">Select a game from the scope list.</p>
{:else}
  <div class="overview">
    <h1>{game.name}</h1>
    <dl>
      <dt>App ID</dt><dd>{game.appId}</dd>
      <dt>Install dir</dt><dd class="mono">{game.installDir}</dd>
      {#if game.prefixPath}
        <dt>Prefix</dt><dd class="mono">{game.prefixPath}</dd>
      {/if}
    </dl>
    <h2>DLL status</h2>
  {#if game.dlls?.length}
      <ul>
        {#each game.dlls as dll}
          <li><strong>{dll.type}</strong> {dll.version || '?'}</li>
        {/each}
      </ul>
    {:else}
      <p class="dim">No DLLs detected</p>
    {/if}
    <p class="launch-hint">Launch via Steam: <code>spela %command%</code></p>
  </div>
{/if}

<style>
  .overview { padding: 1rem 1.25rem; }
  h1 { color: var(--accent-primary); margin-bottom: 1rem; }
  h2 { font-size: 0.8rem; text-transform: uppercase; color: var(--text-dim); margin: 1rem 0 0.5rem; }
  dl { display: grid; grid-template-columns: 6rem 1fr; gap: 0.35rem 1rem; font-size: 0.85rem; }
  .mono { font-family: var(--font-mono); font-size: 0.75rem; word-break: break-all; }
  .dim { color: var(--text-dim); }
  .launch-hint { margin-top: 1.5rem; font-size: 0.8rem; color: var(--text-dim); }
  code { color: var(--accent-focus); }
</style>
