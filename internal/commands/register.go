package commands

import (
	discordx "github.com/discord-bot-framework/kernel-go/internal/discord"
	"github.com/disgoorg/disgo/discord"
)

func Register(r *discordx.Router, s *State) {
	r.Component("/paginator/{id}/{action}", s.handlePaginator)

	r.Slash("/bot debug download", s.handleDebugDownload)
	r.Slash("/bot debug export", s.handleDebugExport)
	r.Autocomplete("/bot debug export", s.autocompleteDebugExport)
	r.Slash("/bot debug info", s.handleDebugInfo)
	r.Slash("/bot debug restart", s.handleDebugRestart)

	r.Slash("/bot kernel info", s.handleKernelInfo)
	r.Slash("/bot kernel update", s.handleKernelUpdate)

	r.Slash("/bot module list", s.handleModuleList)
	r.Slash("/bot module info", s.handleModuleInfo)
	r.Autocomplete("/bot module info", s.autocompleteModuleName)
	r.Slash("/bot module load", s.handleModuleLoad)
	r.Slash("/bot module unload", s.handleModuleUnload)
	r.Autocomplete("/bot module unload", s.autocompleteModuleName)
	r.Slash("/bot module update", s.handleModuleUpdate)
	r.Autocomplete("/bot module update", s.autocompleteModuleName)

	r.Slash("/bot app exec", s.handleAppExec)
	r.Modal("kernel:app:exec", s.handleAppExecModal)
	r.Slash("/bot app info", s.handleAppInfo)
	r.Slash("/bot app search", s.handleAppSearch)
	r.Slash("/bot app scope", s.handleAppScope)
	r.Slash("/bot app delete", s.handleAppDelete)
	r.Autocomplete("/bot app search", s.autocompleteAppCommandID)
	r.Autocomplete("/bot app delete", s.autocompleteAppCommandID)

	r.Slash("/{cmd}", s.handleModuleCommandFallback)
}

func (s *State) CommandDefinitions() []discord.ApplicationCommandCreate {
	core := []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "bot",
			Description: "Bot commands",
			Options: []discord.ApplicationCommandOption{
				debugGroup(),
				kernelGroup(),
				moduleGroup(),
				appGroup(),
			},
		},
	}

	return append(core, s.modules.Commands()...)
}

func debugGroup() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionSubCommandGroup{
		Name:        "debug",
		Description: "Debug commands",
		Options: []discord.ApplicationCommandOptionSubCommand{
			{Name: "download", Description: "Download current code"},
			{
				Name:        "export",
				Description: "Export files",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:         "path",
						Description:  "Relative path to export",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{Name: "info", Description: "Show debugging information"},
			{Name: "restart", Description: "Restart the bot"},
		},
	}
}

func kernelGroup() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionSubCommandGroup{
		Name:        "kernel",
		Description: "Kernel commands",
		Options: []discord.ApplicationCommandOptionSubCommand{
			{Name: "info", Description: "Show information about the kernel"},
			{Name: "update", Description: "Update the kernel to the latest version"},
		},
	}
}

func moduleGroup() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionSubCommandGroup{
		Name:        "module",
		Description: "Module commands",
		Options: []discord.ApplicationCommandOptionSubCommand{
			{
				Name:        "load",
				Description: "Load module from Git URL",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "url",
						Description: "Git repo URL (e.g., https://github.com/user/repo.git)",
						Required:    true,
					},
				},
			},
			{
				Name:        "unload",
				Description: "Unload and delete a module",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:         "module",
						Description:  "Module name",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "update",
				Description: "Update a module to the latest version",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:         "module",
						Description:  "Module name",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "info",
				Description: "Show information about a loaded module",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:         "module",
						Description:  "Module name",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{Name: "list", Description: "List all loaded modules"},
		},
	}
}

func appGroup() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionSubCommandGroup{
		Name:        "app",
		Description: "App commands",
		Options: []discord.ApplicationCommandOptionSubCommand{
			{Name: "exec", Description: "Run arbitrary code"},
			{Name: "info", Description: "Get information about registered app commands"},
			{
				Name:        "search",
				Description: "Search for an application command and export its JSON",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:         "cmd",
						Description:  "Application command ID",
						Required:     true,
						Autocomplete: true,
					},
					discord.ApplicationCommandOptionString{
						Name:        "scope",
						Description: "Scope ID (0 for global)",
						Required:    false,
					},
					discord.ApplicationCommandOptionBool{
						Name:        "remote",
						Description: "Search from Discord API instead of local cache",
						Required:    false,
					},
				},
			},
			{
				Name:        "scope",
				Description: "List commands in a scope",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "scope",
						Description: "Scope ID (0 for global)",
						Required:    false,
					},
				},
			},
			{
				Name:        "delete",
				Description: "Delete command(s) in a scope",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "scope",
						Description: "Scope ID (0 for global)",
						Required:    false,
					},
					discord.ApplicationCommandOptionString{
						Name:         "cmd_id",
						Description:  "Application command ID (leave empty when deleting all)",
						Required:     false,
						Autocomplete: true,
					},
					discord.ApplicationCommandOptionBool{
						Name:        "all",
						Description: "Delete all commands in the selected scope",
						Required:    false,
					},
				},
			},
		},
	}
}
