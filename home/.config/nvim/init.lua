-- ~/.config/nvim/init.lua
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({
    "git",
    "clone",
    "--filter=blob:none",
    "https://github.com/folke/lazy.nvim.git",
    "--branch=stable",
    lazypath,
  })
end
vim.opt.rtp:prepend(lazypath)
require("lazy").setup("pnewsam.plugins")
require("pnewsam.settings")
require("pnewsam.keymap")

-- Enable Zen Mode on startup
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    require("zen-mode").open()
  end,
})
