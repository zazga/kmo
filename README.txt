MATTERMOST KEEP ONLINE — macOS

Wat doet dit?
- Zet je eigen Mattermost-status iedere 240 seconden op "online".
- Draait op de achtergrond met macOS launchd.
- Bewaart je Personal Access Token in de macOS Keychain, niet in een tekstbestand.
- Start automatisch nadat je inlogt op je Mac.

INSTALLEREN
1. Pak dit zip-bestand uit.
2. Open Terminal.
3. Typ: cd gevolgd door een spatie, sleep de map "keep-mattermost-online" in Terminal en druk Enter.
4. Voer uit: chmod +x install.sh uninstall.sh test-connection.sh
5. Voer uit: ./install.sh
6. Vul je Mattermost URL, user ID en Personal Access Token in.

TESTEN
./test-connection.sh

STATUS/LOGS
Bij problemen:
tail -f ~/Library/Logs/keep-mattermost-online.err.log

STOPPEN/VERWIJDEREN
./uninstall.sh

BELANGRIJK
- Je Mattermost-beheerder moet Personal Access Tokens toestaan.
- Sommige organisaties hebben beleid tegen het kunstmatig vasthouden van presence/status. Gebruik dit alleen als dat binnen jouw organisatiebeleid is toegestaan.
- Mattermost of een andere integratie kan je status tussendoor aanpassen; dit script zet hem bij de volgende run opnieuw op online.
