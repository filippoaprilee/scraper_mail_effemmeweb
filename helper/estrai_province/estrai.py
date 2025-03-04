import requests
from bs4 import BeautifulSoup

# URL della pagina Wikipedia della categoria dei comuni di Brindisi
url = "https://it.wikipedia.org/wiki/Categoria:Comuni_della_provincia_di_Taranto"

# Scarica la pagina
response = requests.get(url)
response.raise_for_status()

# Analizza l'HTML con BeautifulSoup
soup = BeautifulSoup(response.text, "html.parser")

comuni = []

# Trova le sezioni con i comuni
category_divs = soup.find_all("div", class_="mw-category-group")

for div in category_divs:
    links = div.find_all("a")
    for link in links:
        comune = link.get_text(strip=True)
        comuni.append(comune)

# Rimuovi eventuali duplicati e ordina
comuni = sorted(set(comuni))

print(f"Trovati {len(comuni)} comuni")

# Scrittura del file CSV
with open("comuni_provincia.csv", "w", newline="", encoding="utf-8") as f:
    f.write("Comune;\n")
    for comune in comuni:
        f.write(f"{comune};\n")

print("CSV creato: comuni_provincia.csv")
