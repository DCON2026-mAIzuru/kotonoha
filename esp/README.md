# ESP Programs

## Files

- `esp_mock_sensor.ino`
  Sends mocked sensor values from ESP32 to `POST /api/v1/sensor`.

## Required settings

Edit these constants before flashing:

```cpp
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";
const char* BACKEND_URL = "http://YOUR_PC_LAN_IP:8080/api/v1/sensor";
const char* SENSOR_TOKEN = "YOUR_SENSOR_SHARED_TOKEN";
const char* SENSOR_USER_ID = "YOUR_SUPABASE_USER_ID";
```

For a fully local setup, `YOUR_PC_LAN_IP` is the Mac/PC address on the same Wi-Fi as the ESP32, for example `10.172.28.184`. Do not use `localhost` or `127.0.0.1` in ESP32 firmware because that would point to the ESP32 itself.

## Related doc

- [docs/setup/step10-esp-mock.md](../docs/setup/step10-esp-mock.md)
