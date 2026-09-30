//! Application state and the pure `update()` function (SPEC §7).
//!
//! `update()` performs no I/O; everything that touches the outside world will be
//! returned as a `Cmd` once there are side effects to perform (M2).

use ratatui::crossterm::event::{KeyCode, KeyEvent, KeyEventKind, KeyModifiers};

use crate::config::Config;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Screen {
    Policies,
    Runs,
    Resources,
    Schema,
    Jobs,
}

impl Screen {
    pub const ALL: [Screen; 5] = [
        Screen::Policies,
        Screen::Runs,
        Screen::Resources,
        Screen::Schema,
        Screen::Jobs,
    ];

    pub fn title(self) -> &'static str {
        match self {
            Screen::Policies => "Policies",
            Screen::Runs => "Runs",
            Screen::Resources => "Resources",
            Screen::Schema => "Schema",
            Screen::Jobs => "Jobs",
        }
    }

    fn index(self) -> usize {
        Self::ALL.iter().position(|s| *s == self).unwrap_or(0)
    }

    fn offset(self, delta: isize) -> Screen {
        let len = Self::ALL.len() as isize;
        Self::ALL[(self.index() as isize + delta).rem_euclid(len) as usize]
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Msg {
    Quit,
    NextScreen,
    PrevScreen,
}

#[derive(Debug)]
pub struct App {
    pub config: Config,
    pub screen: Screen,
    pub should_quit: bool,
}

impl App {
    pub fn new(config: Config) -> Self {
        Self {
            config,
            screen: Screen::Policies,
            should_quit: false,
        }
    }

    pub fn update(&mut self, msg: Msg) {
        match msg {
            Msg::Quit => self.should_quit = true,
            Msg::NextScreen => self.screen = self.screen.offset(1),
            Msg::PrevScreen => self.screen = self.screen.offset(-1),
        }
    }
}

/// Map a key press to a message. Key releases/repeats are ignored.
pub fn msg_for_key(key: KeyEvent) -> Option<Msg> {
    if key.kind != KeyEventKind::Press {
        return None;
    }
    match (key.code, key.modifiers) {
        (KeyCode::Char('q'), _) | (KeyCode::Char('c'), KeyModifiers::CONTROL) => Some(Msg::Quit),
        (KeyCode::Tab, _) | (KeyCode::Char('l'), KeyModifiers::NONE) => Some(Msg::NextScreen),
        (KeyCode::BackTab, _) | (KeyCode::Char('h'), KeyModifiers::NONE) => Some(Msg::PrevScreen),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn press(code: KeyCode) -> KeyEvent {
        KeyEvent::new(code, KeyModifiers::NONE)
    }

    #[test]
    fn starts_on_policies() {
        let app = App::new(Config::default());
        assert_eq!(app.screen, Screen::Policies);
        assert!(!app.should_quit);
    }

    #[test]
    fn screens_cycle_both_ways() {
        let mut app = App::new(Config::default());
        app.update(Msg::PrevScreen);
        assert_eq!(app.screen, Screen::Jobs);
        for _ in 0..Screen::ALL.len() {
            app.update(Msg::NextScreen);
        }
        assert_eq!(app.screen, Screen::Jobs);
    }

    #[test]
    fn key_mapping() {
        assert_eq!(msg_for_key(press(KeyCode::Char('q'))), Some(Msg::Quit));
        assert_eq!(
            msg_for_key(KeyEvent::new(KeyCode::Char('c'), KeyModifiers::CONTROL)),
            Some(Msg::Quit)
        );
        assert_eq!(msg_for_key(press(KeyCode::Tab)), Some(Msg::NextScreen));
        assert_eq!(msg_for_key(press(KeyCode::Char('x'))), None);
    }
}
