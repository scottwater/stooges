# Workspace progress and hook output use diagnostic stderr

Workspace-creation progress and live setup-hook output are diagnostics, so Stooges writes them to stderr—even when the hook wrote to its own stdout—while keeping command stdout and the private auto-CD file protocol unchanged. Engine progress reporting remains optional for compatibility with non-CLI callers; this provides visible feedback without contaminating result output or changing workspace, exit-status, rollback, or auto-CD behavior.
