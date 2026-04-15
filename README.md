# **MomirBox**

MomirBox is a physical, Raspberry Pi-powered Magic: The Gathering "Momir Basic" generator. It uses a high-speed SPI OLED display and a rotary encoder to let users select a Mana Value, randomly selects a valid creature using Scryfall/MTGJSON data, and prints a physical token via a thermal printer.

The project has been rebuilt in **Go** for improved performance, better memory safety when handling large MTGJSON files, and a smoother UI.

## **Hardware Requirements (Production)**

- **Raspberry Pi:** Zero 2 W or similar recommended.
- **Display:** SSD1306 OLED (128x64) using SPI communication.
- **Input:** Rotary Encoder (with 5-button navigation) \+ dedicated Back button.
- **Printer:** Thermal Printer (TTL Serial, 58mm / 384-dot width).

## **Local Development & Desktop Emulator**

You can develop or test the UI on macOS, Windows, or Linux without physical hardware. The Go implementation uses **Ebitengine** to provide a desktop emulator that mirrors the physical OLED's behavior.

### **1\. Install Go**

Ensure you have Go 1.21+ installed on your system.

### **2\. Run the Emulator**

The application automatically detects your platform. If it's not a Raspberry Pi, it launches the emulator:

go run main.go

### **3\. Controls**

- **Left / Right Arrow:** Navigate menus
- **Enter / Space:** Select / Action
- **Backspace:** Go Back
- **Esc:** Exit Emulator

## **Raspberry Pi Deployment**

### **1\. Enable Interfaces**

Run sudo raspi-config on your Pi. Navigate to **Interface Options** and enable:

- **I2C** (for monitoring battery voltages)
- **SPI** (for the Display)
- **Serial Port** (Disable "Login shell over serial", Enable "Serial port hardware")

### **2\. Hardware Configuration**

The application uses BCM pin numbering. Update internal/config/config.go if your wiring differs:

- **SPI:** Standard BCM 7, 8, 9, 10, 11 (CE0, MISO, MOSI, SCLK)
- **Display DC/RST:** Pins 24 and 25
- **Rotary Encoder:** Pins 17 and 27
- **Buttons:** Pins 22 (Select) and 23 (Back)

### **3\. Build and Run**

On the Pi, compile the binary for maximum performance:

```
go build \-o momirbox main.go
./momirbox
```

## **First Run Instructions**

Once the app is running, you must synchronize the local database and assets via the **Settings** menu:

1. **Update DB:** Downloads the latest AtomicCards.json from MTGJSON (\~130MB).
2. **Sync Images:** Downloads \~17,000 creature images from Scryfall.
   - _Note: This respects Scryfall's rate limits (110ms delay) and takes approximately one hour on a fresh install._
3. **Sync Tokens (Optional):** If enabled in Preferences, grabs the token and emblem library.

**Offline Usage:** An internet connection is only required for these initial setup steps. Once cached, MomirBox is entirely self-contained.

## **Running on Boot (systemd)**

To make MomirBox start automatically on power-up:

1. Create a service file:

```bash
sudo nano /etc/systemd/system/momirbox.service
```

2. Paste the following (adjust paths to your project directory):

```
[Unit]
Description=MomirBox Go Service
After=network.target

[Service]
ExecStart=/home/pi/momirbox/momirbox
WorkingDirectory=/home/pi/momirbox
StandardOutput=inherit
StandardError=inherit
Restart=always
User=pi

[Install]
WantedBy=multi-user.target
```

3. Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable momirbox.service
sudo systemctl start momirbox.service
```

## **Technical Details**

- **Architecture:** Decoupled hardware interfaces (OLED/SPI, GPIO, Serial) for easy swapping between physical hardware and emulation.
- **Streaming Parser:** Uses a custom JSON stream-decoder to parse massive MTGJSON files on the Pi Zero's limited RAM without crashing.
- **Cinematics:** Smooth 60FPS animations and GIF playback powered by a custom Go rendering engine.
