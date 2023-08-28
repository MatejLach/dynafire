dynafire - real-time threat detection for any Linux system
=

[Turris Sentinel](https://view.sentinel.turris.cz/?period=1w) is a real-time threat detection & attack prevention system from
the creators of the [Turris](https://www.turris.com/en/) series of open-source routers, however this service is normally only available via the router interface.
This makes it impractical to use the real-time data provided by Turris Sentinel on a VPS for example, which you cannot easily put behind a Turris router hardware.

`dynafire` is a lightweight Linux daemon that lets any Linux system running the industry standard `firewalld` firewall update its firewall rules in real-time based on Sentinel data.

Turris Sentinel data by TurrisTech is licensed under a Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License.

Installation via package managers
-
Arch Linux (AUR): [dynafire-bin](https://aur.archlinux.org/packages/dynafire-bin)

Manual installation
-
Because `dynafire` ships as a single binary, it is easy to install it manually on practically any `systemd`-based distro.

Before proceeding please ensure that both `NetworkManager` and `firewalld` are installed and running:

```shell
$ sudo systemctl check NetworkManager                                   
active

$ sudo systemctl check firewalld                                   
active
```

Download the binary:

`$ wget https://github.com/MatejLach/dynafire/releases/download/v0.1/dynafire`

Ensure the binary is executable:

`$ chmod +x dynafire`

Copy the binary to your `$PATH`:

`$ sudo cp dynafire /usr/bin/`


Next, download the `systemd` service definition file:

`$ wget https://raw.githubusercontent.com/MatejLach/dynafire/main/dist/systemd/dynafire.service` 

Copy it under where `systemd` would be able to see it i.e. `/lib/systemd/system` or `/etc/systemd/system`:

`$ sudo cp dynafire.service /lib/systemd/system/`

Register the new service with `systemd`:

`$ sudo systemctl daemon-reload`

Then, assuming `firewalld` is already running, enable it at boot and start with:

`$ sudo systemctl enable dynafire --now`

Building from source
-
Clone the source:

`$ git clone https://github.com/MatejLach/dynafire.git && cd dynafire/cmd/dynafire`

Then, assuming a properly [set up Go toolchain](https://golang.org/doc/install), simply run:

`$ go build`

Copy the resulting `dynafire` binary under `/usr/bin` and use the [systemd service](dist/systemd/dynafire.service) to manage its lifecycle, see [Manual Installation](#manual-installation) for details.

Configuration
-
The `dynafire` configuration file is created upon first launch under `/etc/dynafire/config.json`.
By default, it has the following values:

```json
{
  "log_level": "INFO",
  "zone_target_policy": "ACCEPT"
}
```

The `log_level` can be set to `DEBUG` (most verbose), `INFO` and `ERROR` (least verbose).

By default, the `dynafire` firewalld zone is set to `ACCEPT` every packet that is NOT on the Turris Sentinel blacklist, so as not to accidentally block legitimate traffic. 
However, you can make this stricter by changing the `zone_target_policy` to i.e. `REJECT` or `DROP`, see [firewalld zone options](https://firewalld.org/documentation/zone/options.html) for details.  

Contributing
-
Bug reports and pull requests are welcome. Do not hesitate to open a PR / file an issue or a feature request.
