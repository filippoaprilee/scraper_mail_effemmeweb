import requests
from bs4 import BeautifulSoup

# URL di esempio: pagina Wikipedia che elenca i comuni della Lombardia
url = "https://it.wikipedia.org/wiki/Comuni_della_Valle_d%27Aosta"  # Controlla che l'URL sia corretto

response = requests.get(url)
response.raise_for_status()  # Controlla eventuali errori

soup = BeautifulSoup(response.text, "html.parser")

comuni = []

# Adatta il selettore in base alla struttura della pagina
tables = soup.find_all("table", {"class": "wikitable"})
for table in tables:
    rows = table.find_all("tr")
    for row in rows[1:]:  # saltiamo l'intestazione
        cells = row.find_all(["th", "td"])
        if cells:
            comune = cells[0].get_text(strip=True)
            if comune and comune not in comuni:
                comuni.append(comune)

print(f"Trovati {len(comuni)} comuni")

# Scrittura del file CSV con il punto e virgola alla fine di ogni riga
with open("comuni_lombardia.csv", "w", newline="", encoding="utf-8") as f:
    # Scrivi l'intestazione e aggiungi un punto e virgola finale
    f.write("Comune;" + "\n")
    # Scrivi ogni comune su una riga, aggiungendo sempre il punto e virgola alla fine
    for comune in comuni:
        f.write(f"{comune};" + "\n")

print("CSV creato: comuni_lombardia.csv")
