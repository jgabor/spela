# TechPowerUp download page fixtures

Pages in the shape `dlss-updater` reads: the version in the `<title>`, and the
file id in the download form's hidden `id` input. They are written by hand to
the patterns of Go's `tools/dlss-updater`, not saved from techpowerup.com,
because tests and agents never reach the network. Replace one with a saved page
when the real layout changes.
