<script>
  import '$lib/neumorphic.css';
  import '$lib/styles.css';
  let { children } = $props();
</script>

<div class="neumorph-body">
  {@render children()}
</div>
