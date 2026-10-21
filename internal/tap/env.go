package tap

import (
	"fmt"
	"io"
	"strings"
)

// Shells that `acurl tap env` can write wrappers for.
var Shells = []string{"bash", "zsh", "fish", "powershell"}

// WriteEnv writes shell functions that run each CLI through the proxy at
// base (e.g. "http://127.0.0.1:8899") while `acurl tap` is running, and
// straight to the provider when it is not, so the CLI never breaks.
func WriteEnv(w io.Writer, shell, base string, clis []CLI) error {
	base = strings.TrimSuffix(base, "/")
	ping := base + PingPath
	fmt.Fprintf(w, "# acurl tap: route AI CLI traffic through %s while `acurl tap` is running\n", base)
	for _, c := range clis {
		var vars [][2]string
		for _, b := range c.Bindings {
			r, ok := findRoute(DefaultRoutes, b.Route)
			if !ok {
				return fmt.Errorf("%s: unknown route %q", c.Name, b.Route)
			}
			vars = append(vars, [2]string{b.Env, base + r.Prefix + b.Suffix})
		}
		switch shell {
		case "bash", "zsh":
			var assign []string
			for _, v := range vars {
				assign = append(assign, v[0]+"='"+v[1]+"'")
			}
			fmt.Fprintf(w, "%s() { if curl -fsS -m 1 '%s' >/dev/null 2>&1; then %s command %s \"$@\"; else command %s \"$@\"; fi; }\n",
				c.Name, ping, strings.Join(assign, " "), c.Name, c.Name)
		case "fish":
			var assign []string
			for _, v := range vars {
				assign = append(assign, v[0]+"='"+v[1]+"'")
			}
			fmt.Fprintf(w, "function %s\n    if curl -fsS -m 1 '%s' >/dev/null 2>&1\n        env %s %s $argv\n    else\n        command %s $argv\n    end\nend\n",
				c.Name, ping, strings.Join(assign, " "), c.Name, c.Name)
		case "powershell":
			// Save each variable and put it back afterwards, so a value the
			// user set themselves survives the call.
			var save, set, restore []string
			for i, v := range vars {
				save = append(save, fmt.Sprintf("$old%d = $env:%s", i, v[0]))
				set = append(set, "$env:"+v[0]+" = '"+v[1]+"'")
				restore = append(restore, fmt.Sprintf("$env:%s = $old%d", v[0], i))
			}
			fmt.Fprintf(w, "function %s {\n    $exe = (Get-Command %s -CommandType Application | Select-Object -First 1).Source\n"+
				"    try { $null = Invoke-WebRequest -UseBasicParsing -TimeoutSec 1 '%s'; $on = $true } catch { $on = $false }\n"+
				"    if (-not $on) { & $exe @args; return }\n    %s\n    %s\n    try { & $exe @args } finally { %s }\n}\n",
				c.Name, c.Name, ping, strings.Join(save, "; "), strings.Join(set, "; "), strings.Join(restore, "; "))
		default:
			return fmt.Errorf("unsupported shell %q; use one of %s", shell, strings.Join(Shells, ", "))
		}
	}
	return nil
}
