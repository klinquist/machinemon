"""Constants for the MachineMon integration."""

from datetime import timedelta

DOMAIN = "machinemon"
RUNTIME = "runtime"

CONF_COLLECTION_URL = "collection_url"
CONF_VERIFY_SSL = "verify_ssl"
CONF_CLIENT_ID = "client_id"

DEFAULT_CHECKIN_INTERVAL = timedelta(seconds=120)
