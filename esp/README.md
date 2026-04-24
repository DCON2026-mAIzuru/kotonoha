# ESP Programs

## Files

- `esp_mock_sensor.ino`
  Sends mocked sensor values from ESP32 to `POST /api/v1/sensor`.

## Required settings

Edit these constants before flashing:

```cpp
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";
const char* BACKEND_URL = "https://YOUR_BACKEND_URL/api/v1/sensor";
const char* SENSOR_TOKEN = "YOUR_SENSOR_SHARED_TOKEN";
const char* SENSOR_USER_ID = "YOUR_SUPABASE_USER_ID";
```

## Related doc

- [docs/setup/step10-esp-mock.md](/Users/apple/Documents/teamproject/kotonoha/docs/setup/step10-esp-mock.md:1)
