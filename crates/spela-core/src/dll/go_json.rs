//! Reading JSON the way Go's `encoding/json` reads it into Go spela's
//! structs, for the files Go wrote: `backup.json` and the manifest.
//!
//! - A missing key or a `null` is the zero value, where serde would fail on
//!   `null` for a string, a list or a map.
//! - A Go `time.Time` is RFC 3339. Go's zero time, `0001-01-01T00:00:00Z`,
//!   is what Go writes for a time it never set, so it reads as `None`.

use jiff::Timestamp;
use serde::{Deserialize, Deserializer};

/// Go's zero `time.Time`, 0001-01-01T00:00:00Z.
pub(crate) const GO_ZERO_TIME_SECONDS: i64 = -62_135_596_800;

/// A value whose `null` reads as its default.
pub(crate) fn null_default<'de, D, T>(deserializer: D) -> Result<T, D::Error>
where
    D: Deserializer<'de>,
    T: Default + Deserialize<'de>,
{
    Ok(Option::<T>::deserialize(deserializer)?.unwrap_or_default())
}

/// Whether a time is Go's zero time.
pub(crate) fn is_go_zero(time: Timestamp) -> bool {
    time.as_second() == GO_ZERO_TIME_SECONDS && time.subsec_nanosecond() == 0
}

/// Parses a time as Go's `time.Time` JSON form, with the zero time as `None`.
pub(crate) fn parse_time(text: &str) -> Result<Option<Timestamp>, jiff::Error> {
    let time: Timestamp = text.parse()?;
    Ok((!is_go_zero(time)).then_some(time))
}

/// serde `with` module for an optional Go time.
pub(crate) mod time {
    use jiff::Timestamp;
    use serde::{Deserialize, Deserializer, Serializer};

    pub(crate) fn serialize<S: Serializer>(
        time: &Option<Timestamp>,
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        match time {
            Some(time) => serializer.collect_str(time),
            None => serializer.serialize_none(),
        }
    }

    pub(crate) fn deserialize<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Option<Timestamp>, D::Error> {
        match Option::<String>::deserialize(deserializer)? {
            None => Ok(None),
            Some(text) => super::parse_time(&text).map_err(serde::de::Error::custom),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Deserialize)]
    struct Probe {
        #[serde(default, deserialize_with = "null_default")]
        name: String,
        #[serde(default, deserialize_with = "null_default")]
        list: Vec<u32>,
        #[serde(default, with = "time")]
        at: Option<Timestamp>,
    }

    #[test]
    fn null_and_missing_are_zero() {
        let p: Probe = serde_json::from_str(r#"{"name":null,"list":null,"at":null}"#).unwrap();
        assert_eq!((p.name.as_str(), p.list.len(), p.at), ("", 0, None));
        let p: Probe = serde_json::from_str("{}").unwrap();
        assert_eq!((p.name.as_str(), p.list.len(), p.at), ("", 0, None));
    }

    #[test]
    fn go_times_read_with_any_offset_and_zero_is_none() {
        let p: Probe =
            serde_json::from_str(r#"{"at":"2026-10-26T17:02:10.123456789+02:00"}"#).unwrap();
        assert_eq!(
            p.at,
            Some("2026-10-26T15:02:10.123456789Z".parse().unwrap())
        );
        for zero in ["0001-01-01T00:00:00Z", "0001-01-01T02:00:00+02:00"] {
            assert_eq!(parse_time(zero).unwrap(), None, "{zero}");
        }
        assert!(serde_json::from_str::<Probe>(r#"{"at":"yesterday"}"#).is_err());
        assert!(serde_json::from_str::<Probe>(r#"{"at":""}"#).is_err());
    }
}
