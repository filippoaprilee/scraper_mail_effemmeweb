import requests
from bs4 import BeautifulSoup

# URL della pagina Wikipedia
url = "https://it.wikipedia.org/wiki/Comuni_del_Trentino-Alto_Adige"

response = requests.get(url)
response.raise_for_status()  # Controlla eventuali errori

soup = BeautifulSoup(response.text, "html.parser")

comuni = []

# Trova tutte le tabelle con classe "wikitable"
tables = soup.find_all("table", {"class": "wikitable"})
for table in tables:
    rows = table.find_all("tr")
    for row in rows[1:]:  # Salta la riga dell'intestazione
        cells = row.find_all("td")
        if cells:
            # Supponendo che il nome del comune sia nella prima colonna (indice 0)
            comune = cells[0].get_text(strip=True)
            # Aggiungi il nome del comune se è valido (non un numero)
            if comune and not comune.isdigit():
                comuni.append(comune)

# Rimuovi eventuali duplicati
comuni = list(set(comuni))

print(f"Trovati {len(comuni)} comuni")

# Scrittura del file CSV con il punto e virgola alla fine di ogni riga
with open("lista_comuni.csv", "w", newline="", encoding="utf-8") as f:
    # Scrivi l'intestazione
    f.write("Comune;" + "\n")
    # Scrivi ogni comune su una riga, aggiungendo sempre il punto e virgola alla fine
    for comune in sorted(comuni):
        f.write(f"{comune};" + "\n")

print("CSV creato: lista_comuni.csv")
