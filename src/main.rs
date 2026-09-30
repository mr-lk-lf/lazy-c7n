mod app;
mod config;
mod ui;

use std::path::PathBuf;

use anyhow::Result;
use clap::Parser;
use ratatui::DefaultTerminal;
use ratatui::crossterm::event::{self, Event};

use crate::app::{App, msg_for_key};

/// A terminal UI for the Cloud Custodian (`custodian`) CLI.
///
/// Independent project; not affiliated with Cloud Custodian or the CNCF.
#[derive(Debug, Parser)]
#[command(version, about)]
struct Cli {
    /// Config file to use instead of the user config + `.lazyc7n.toml`.
    #[arg(long, short)]
    config: Option<PathBuf>,
}

fn main() -> Result<()> {
    let cli = Cli::parse();
    let config = config::load(cli.config.as_deref(), &std::env::current_dir()?)?;
    let mut app = App::new(config);
    // ratatui::run() sets up the terminal, installs a panic hook that restores it,
    // and restores it again on return.
    ratatui::run(|terminal| run(terminal, &mut app))
}

fn run(terminal: &mut DefaultTerminal, app: &mut App) -> Result<()> {
    while !app.should_quit {
        terminal.draw(|frame| ui::view(app, frame))?;
        if let Event::Key(key) = event::read()?
            && let Some(msg) = msg_for_key(key)
        {
            app.update(msg);
        }
    }
    Ok(())
}
