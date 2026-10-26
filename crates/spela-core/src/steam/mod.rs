//! Steam: library folders, app manifests and the VDF they are written in.
//!
//! | Module | Role |
//! |---|---|
//! | [`vdf`] | VDF text on keyvalues-parser, last duplicate key wins |
//! | [`manifest`] | `appmanifest_<appid>.acf` and the installed bit |
//! | [`library`] | The Steam root, `libraryfolders.vdf`, extra libraries and prefixes |
//! | [`tools`] | Proton, runtimes and other Steam tools, which are not games |
//! | [`detect`] | Which game a wrapped command or a running process belongs to |

pub mod detect;
pub mod library;
pub mod manifest;
pub mod tools;
pub mod vdf;
