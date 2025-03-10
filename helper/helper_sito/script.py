import requests
import json
import socket
import dns.resolver
from bs4 import BeautifulSoup

# Configurazione API (aggiungi le tue chiavi API se necessario)
SHODAN_API_KEY = "YgCCsdRgTvTDBVUUr4Q5A4vGjjf4CjIG"
PAGESPEED_API_KEY = "AIzaSyD13bhKEEwzY15yMgsolkVvMCuToZsHPlU"

def get_hosting_provider(domain):
    try:
        ip_address = socket.gethostbyname(domain)
        url = f"https://api.shodan.io/shodan/host/{ip_address}?key={SHODAN_API_KEY}"
        response = requests.get(url)
        data = response.json()
        return data.get("isp", "N/A")
    except Exception as e:
        return str(e)

def get_dns_records(domain):
    records = {}
    try:
        for record_type in ["A", "NS", "MX", "TXT"]:
            records[record_type] = [str(r) for r in dns.resolver.resolve(domain, record_type)]
    except Exception as e:
        records["error"] = str(e)
    return records

def get_technologies(url):
    try:
        response = requests.get(url, headers={"User-Agent": "Mozilla/5.0"})
        soup = BeautifulSoup(response.text, 'html.parser')
        technologies = set()
        
        # Rileva CMS
        meta_generator = soup.find("meta", attrs={"name": "generator"})
        if meta_generator:
            technologies.add(meta_generator["content"])
        
        # Rileva framework JS
        scripts = [script["src"] for script in soup.find_all("script", src=True)]
        if any("/wp-includes/" in s or "/wp-content/" in s for s in scripts):
            technologies.add("WordPress")
        if any("/media/system/js/" in s for s in scripts):
            technologies.add("Joomla")
        if any("/sites/all/modules/" in s for s in scripts):
            technologies.add("Drupal")
        
        # Rileva librerie JS
        if any("jquery" in s.lower() for s in scripts):
            technologies.add("jQuery")
        if any("angular" in s.lower() for s in scripts):
            technologies.add("AngularJS")
        if any("react" in s.lower() for s in scripts):
            technologies.add("ReactJS")
        if any("vue" in s.lower() for s in scripts):
            technologies.add("Vue.js")
        
        return list(technologies) if technologies else "Nessuna tecnologia rilevata"
    except Exception as e:
        return str(e)

def get_performance(url):
    performance = {}
    try:
        for strategy in ["desktop", "mobile"]:
            api_url = f"https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url={url}&strategy={strategy}&key={PAGESPEED_API_KEY}"
            response = requests.get(api_url)
            data = response.json()
            performance[strategy] = data.get("lighthouseResult", {}).get("categories", {}).get("performance", {}).get("score", "N/A")
    except Exception as e:
        performance["error"] = str(e)
    return performance

def get_wordpress_details(url):
    details = {"theme": None, "plugins": []}
    try:
        response = requests.get(url, headers={"User-Agent": "Mozilla/5.0"})
        soup = BeautifulSoup(response.text, 'html.parser')
        
        # Trova il tema WordPress
        theme_link = soup.find("link", href=lambda x: x and "wp-content/themes" in x)
        if theme_link:
            details["theme"] = theme_link["href"].split("/themes/")[-1].split("/")[0]
        
        # Trova i plugin di WordPress
        plugins = set()
        for script in soup.find_all("script", src=True):
            src = script["src"]
            if "wp-content/plugins" in src:
                plugin_name = src.split("/plugins/")[-1].split("/")[0]
                plugins.add(plugin_name)
        for link in soup.find_all("link", href=True):
            href = link["href"]
            if "wp-content/plugins" in href:
                plugin_name = href.split("/plugins/")[-1].split("/")[0]
                plugins.add(plugin_name)
        
        details["plugins"] = list(plugins) if plugins else "Nessun plugin rilevato"
    except Exception as e:
        details["error"] = str(e)
    return details

def get_prestashop_modules(url):
    modules = []
    try:
        response = requests.get(url)
        soup = BeautifulSoup(response.text, 'html.parser')
        for script in soup.find_all("script", src=True):
            if "modules" in script["src"]:
                modules.append(script["src"])
    except Exception as e:
        modules.append(str(e))
    return modules

def main():
    url = input("Inserisci l'URL del sito: ")
    domain = url.replace("https://", "").replace("http://", "").split("/")[0]
    
    print("\n--- Raccolta informazioni ---")
    print(f"Hosting Provider: {get_hosting_provider(domain)}")
    print(f"DNS Records: {json.dumps(get_dns_records(domain), indent=2)}")
    print(f"Tecnologie utilizzate: {get_technologies(url)}")
    print(f"Performance: {json.dumps(get_performance(url), indent=2)}")
    print(f"Dettagli WordPress: {json.dumps(get_wordpress_details(url), indent=2)}")
    print(f"Moduli PrestaShop: {json.dumps(get_prestashop_modules(url), indent=2)}")

if __name__ == "__main__":
    main()