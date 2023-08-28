dynafire - real-time threat detection for any Linux system
=

[Turris Sentinel](https://view.sentinel.turris.cz/?period=1w) is a real-time threat detection & attack prevention system from
the creators of the [Turris](https://www.turris.com/en/) series of open-source routers, however this service is normally only available via the router interface.
This makes it impractical to use the real-time data provided by Turris Sentinel on a VPS for example, which you cannot easily put behind a Turris router hardware.

`dynafire` is a lightweight Linux daemon that lets any Linux system running the industry standard `firewalld` firewall update its firewall rules in real-time based on Sentinel data.

Installation via package managers
-
TODO

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

TODO

Ensure the binary is executable:

`$ chmod +x dynafire`

Copy the binary to your `$PATH`:

`$ sudo cp dynafire /usr/bin/`


Next, download the `systemd` service definition file:

TODO 

Building from source
-

TODO

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
