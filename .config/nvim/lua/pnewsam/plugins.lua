-- ~/.config/nvim/lua/paulnewsam/plugins.lua

return {
  -- Treesitter for syntax highlighting
  {
    "nvim-treesitter/nvim-treesitter",
    build = ":TSUpdate", -- Updates Treesitter parsers on install/update
    config = function()
      require("nvim-treesitter.configs").setup({
        -- Enable syntax highlighting
        highlight = { enable = true },
        -- Automatically install parsers for these languages
        ensure_installed = { "lua", "vim", "vimdoc", "javascript", "typescript", "python", "json", "tsx" }, -- Add more languages as needed
        -- Install parsers synchronously (only for ensure_installed)
        sync_install = false,
      })
    end,
  },
  -- Tokyo Night theme
  {
    "folke/tokyonight.nvim",
    lazy = false, -- Load immediately to apply the theme on startup
    priority = 1000, -- Ensure it loads before other plugins that might depend on colors
    config = function()
      -- Optional: Customize the theme (defaults work fine too)
      require("tokyonight").setup({
        style = "night", -- Options: "storm", "moon", "night", "day"
        transparent = false, -- Set to true for transparent background
        terminal_colors = true, -- Apply theme to terminal colors
        styles = {
          comments = { italic = true },
          keywords = { italic = true },
        },
      })
      -- Set the colorscheme
      vim.cmd("colorscheme tokyonight")
    end,
  },
  {
	'nvim-telescope/telescope.nvim', tag = '0.1.8',
	dependencies = { 'nvim-lua/plenary.nvim' }
  },
  {
	"folke/zen-mode.nvim",
	opts = {},
}
}
