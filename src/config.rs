//! Configuration loading (SPEC §5).
//!
//! The user config (`$XDG_CONFIG_HOME/lazyc7n/config.toml`) is deep-merged with an
//! optional project-local `.lazyc7n.toml`; keys in the project file win.

use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde::Deserialize;

pub const PROJECT_FILE: &str = ".lazyc7n.toml";

#[derive(Debug, Clone, PartialEq, Deserialize)]
#[serde(default)]
pub struct Config {
    pub policy_dirs: Vec<PathBuf>,
    pub runner: RunnerConfig,
    pub defaults: Defaults,
    pub safety: Safety,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            policy_dirs: vec![PathBuf::from("./policies")],
            runner: RunnerConfig::default(),
            defaults: Defaults::default(),
            safety: Safety::default(),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RunnerKind {
    #[default]
    Binary,
    Docker,
    Command,
}

#[derive(Debug, Clone, PartialEq, Deserialize)]
#[serde(default)]
pub struct RunnerConfig {
    pub kind: RunnerKind,
    pub custodian: String,
    pub command: Vec<String>,
}

impl Default for RunnerConfig {
    fn default() -> Self {
        Self {
            kind: RunnerKind::Binary,
            custodian: "custodian".into(),
            command: Vec::new(),
        }
    }
}

#[derive(Debug, Clone, Default, PartialEq, Deserialize)]
#[serde(default)]
pub struct Defaults {
    /// Empty = let custodian decide.
    pub region: String,
    pub cache_period: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum ConfirmLive {
    #[default]
    TypeName,
    YesNo,
}

#[derive(Debug, Clone, PartialEq, Deserialize)]
#[serde(default)]
pub struct Safety {
    pub default_dry_run: bool,
    pub confirm_live: ConfirmLive,
}

impl Default for Safety {
    fn default() -> Self {
        Self {
            default_dry_run: true,
            confirm_live: ConfirmLive::TypeName,
        }
    }
}

/// `$XDG_CONFIG_HOME/lazyc7n/config.toml` (platform equivalent elsewhere).
pub fn user_config_path() -> Option<PathBuf> {
    directories::ProjectDirs::from("", "", "lazyc7n").map(|d| d.config_dir().join("config.toml"))
}

/// Load `explicit` if given (must exist), otherwise the user config merged with
/// `<cwd>/.lazyc7n.toml`. Missing implicit files are not an error.
pub fn load(explicit: Option<&Path>, cwd: &Path) -> Result<Config> {
    let files: Vec<PathBuf> = match explicit {
        Some(path) => {
            anyhow::ensure!(path.exists(), "config file not found: {}", path.display());
            vec![path.to_path_buf()]
        }
        None => user_config_path()
            .into_iter()
            .chain([cwd.join(PROJECT_FILE)])
            .filter(|p| p.exists())
            .collect(),
    };

    let mut merged = toml::Table::new();
    for file in &files {
        let text =
            fs::read_to_string(file).with_context(|| format!("reading {}", file.display()))?;
        let table: toml::Table =
            toml::from_str(&text).with_context(|| format!("parsing {}", file.display()))?;
        merge(&mut merged, table);
    }
    toml::Value::Table(merged)
        .try_into()
        .context("invalid configuration")
}

fn merge(base: &mut toml::Table, over: toml::Table) {
    for (key, value) in over {
        match (base.get_mut(&key), value) {
            (Some(toml::Value::Table(b)), toml::Value::Table(o)) => merge(b, o),
            (_, v) => {
                base.insert(key, v);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_config_is_safe_default() {
        let cfg: Config = toml::from_str("").unwrap();
        assert_eq!(cfg, Config::default());
        assert!(cfg.safety.default_dry_run);
        assert_eq!(cfg.safety.confirm_live, ConfirmLive::TypeName);
    }

    #[test]
    fn unknown_keys_are_ignored() {
        let cfg: Config =
            toml::from_str("future_key = 1\n[runner]\nkind = \"docker\"\nx = 2\n").unwrap();
        assert_eq!(cfg.runner.kind, RunnerKind::Docker);
    }

    #[test]
    fn project_file_overrides_nested_keys_only() {
        let dir = tempfile::tempdir().unwrap();
        let user = dir.path().join("user.toml");
        fs::write(
            &user,
            "policy_dirs = [\"a\"]\n[runner]\ncustodian = \"/venv/bin/custodian\"\n",
        )
        .unwrap();
        let mut merged =
            toml::from_str::<toml::Table>(&fs::read_to_string(&user).unwrap()).unwrap();
        merge(
            &mut merged,
            toml::from_str("[runner]\nkind = \"command\"\n").unwrap(),
        );
        let cfg: Config = toml::Value::Table(merged).try_into().unwrap();
        assert_eq!(cfg.policy_dirs, vec![PathBuf::from("a")]);
        assert_eq!(cfg.runner.kind, RunnerKind::Command);
        assert_eq!(cfg.runner.custodian, "/venv/bin/custodian");
    }

    #[test]
    fn explicit_missing_file_is_an_error() {
        let dir = tempfile::tempdir().unwrap();
        let err = load(Some(&dir.path().join("nope.toml")), dir.path()).unwrap_err();
        assert!(err.to_string().contains("not found"));
    }

    #[test]
    fn explicit_file_is_loaded() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("c.toml");
        fs::write(&path, "[safety]\nconfirm_live = \"yes-no\"\n").unwrap();
        let cfg = load(Some(&path), dir.path()).unwrap();
        assert_eq!(cfg.safety.confirm_live, ConfirmLive::YesNo);
    }
}
