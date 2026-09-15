MATTERMOST KEEP ONLINE — macOS

Wat doet dit?
- Zet je eigen Mattermost-status iedere 240 seconden op "online".
- Draait op de achtergrond met macOS launchd.
- Bewaart je Personal Access Token in de macOS Keychain, niet in een tekstbestand.
- Haalt je interne Mattermost user ID automatisch op via /api/v4/users/me.
- Start automatisch nadat je inlogt op je Mac.

INSTALLEREN
1. Open Terminal.
2. Ga naar deze repository/map.
3. Voer uit: chmod +x install.sh uninstall.sh test-connection.sh
4. Voer uit: ./install.sh
5. Vul je Mattermost URL en Personal Access Token in.
6. De installer controleert het token en bepaalt automatisch je Mattermost user ID.

TESTEN
./test-connection.sh

De test controleert zowel /api/v4/users/me als de status-update naar "online".

STATUS/LOGS
Bij problemen:
tail -f ~/Library/Logs/keep-mattermost-online.err.log

STOPPEN/VERWIJDEREN
./uninstall.sh

BELANGRIJK
- Je Mattermost-beheerder moet Personal Access Tokens toestaan.
- Sommige organisaties hebben beleid tegen het kunstmatig vasthouden van presence/status. Gebruik dit alleen als dat binnen jouw organisatiebeleid is toegestaan.
- Mattermost of een andere integratie kan je status tussendoor aanpassen; dit script zet hem bij de volgende run opnieuw op online.
