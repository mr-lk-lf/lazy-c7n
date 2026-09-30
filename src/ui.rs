//! Pure render functions: `view(&App, &mut Frame)`.

use ratatui::Frame;
use ratatui::layout::{Constraint, Layout};
use ratatui::style::{Color, Modifier, Style, Stylize};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, List, Paragraph, Tabs};

use crate::app::{App, Screen};

pub fn view(app: &App, frame: &mut Frame) {
    let [header, body, footer] = Layout::vertical([
        Constraint::Length(1),
        Constraint::Min(0),
        Constraint::Length(1),
    ])
    .areas(frame.area());

    let tabs = Tabs::new(Screen::ALL.iter().map(|s| s.title()))
        .select(Screen::ALL.iter().position(|s| *s == app.screen))
        .highlight_style(Style::new().add_modifier(Modifier::BOLD | Modifier::REVERSED));
    frame.render_widget(tabs, header);

    let block = Block::bordered().title(app.screen.title());
    match app.screen {
        Screen::Policies => {
            let items = app
                .config
                .policy_dirs
                .iter()
                .map(|d| d.display().to_string());
            frame.render_widget(
                List::new(items).block(block.title_bottom("policy dirs")),
                body,
            );
        }
        _ => frame.render_widget(Paragraph::new("not implemented yet").block(block), body),
    }

    frame.render_widget(status_line(app), footer);
}

fn status_line(app: &App) -> Line<'static> {
    let mode = if app.config.safety.default_dry_run {
        Span::styled(" DRY ", Style::new().bg(Color::Green).fg(Color::Black))
    } else {
        Span::styled(
            " LIVE ",
            Style::new().bg(Color::Red).fg(Color::White).bold(),
        )
    };
    Line::from(vec![
        mode,
        Span::raw(format!(" runner: {} ", app.config.runner.custodian)),
        Span::raw("│ tab/h/l screens │ q quit").dim(),
    ])
}

#[cfg(test)]
mod tests {
    use ratatui::Terminal;
    use ratatui::backend::TestBackend;

    use super::*;
    use crate::config::Config;

    fn render(app: &App) -> String {
        let mut terminal = Terminal::new(TestBackend::new(60, 8)).unwrap();
        terminal.draw(|f| view(app, f)).unwrap();
        let buffer = terminal.backend().buffer();
        buffer
            .content()
            .chunks(buffer.area.width as usize)
            .map(|row| row.iter().map(|c| c.symbol()).collect::<String>())
            .collect::<Vec<_>>()
            .join("\n")
    }

    #[test]
    fn policies_screen_lists_dirs_and_shows_dry_badge() {
        let out = render(&App::new(Config::default()));
        assert!(out.contains("./policies"), "{out}");
        assert!(out.contains(" DRY "), "{out}");
        assert!(!out.contains("LIVE"), "{out}");
    }

    #[test]
    fn live_default_is_visible() {
        let mut config = Config::default();
        config.safety.default_dry_run = false;
        assert!(render(&App::new(config)).contains(" LIVE "));
    }
}
