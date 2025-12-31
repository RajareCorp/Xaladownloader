# 📦 XalaDownloader

Un petit serveur Go qui recherche et télécharge les vidéos de Xalaflix.

**Auteur :** Rajare  
**Version :** v1.0.0 (début 2025)

---

## 🎯 Qu’est‑ce que c’est ?

XalaDownloader expose une API REST et une interface web sombre permettant :

| Fonctionnalité | Description |
|----------------|------------|
| 🔎 Recherche | Interroge `https://xalaflix.men/search_elastic?s=…` (ou toute autre URL configurable) et renvoie les titres, miniatures et liens de détail. |
| 🗑️ Déduplication | Élimine les doublons d’URL/titres avant de les renvoyer au client. |
| 📥 Téléchargement | Proxie le flux MP4 en ajoutant `Content-Disposition: attachment; filename="<titre>.mp4"` afin que le navigateur télécharge le fichier avec le bon nom. |
| ⚙️ Configuration dynamique | L’URL de base de Xalaflix peut être changée à la volée via `/admin/base-url` (persistée dans `config.json`). |
| 🌙 Interface sombre | UI responsive, cartes de résultats avec effets de hover, barre de progression intégrée. |

---

## 📋 Table des matières

1. Prérequis
2. Installation
3. Configuration
4. Lancement
5. Endpoints API
6. Licence
7. Contact

---

## 🔧 Prérequis

| Outil | Version minimale |
|-------|----------------|
| Go | 1.22 (ou plus récent) |
| Git | 2.x (pour cloner le dépôt) |
| Internet | Accès à `xalaflix.men` (ou à l’URL que vous configurez) |

**NOTE :** Le serveur ne nécessite aucune base de données, tout est stocké dans le fichier `config.json`.

---

## ⬇️ Installation

### 1️⃣ Cloner le dépôt
```bash
git clone https://github.com/rajare/xaladownloader.git
cd xaladownloader
```

# 2️⃣ Télécharger les dépendances Go (goquery)
```bash
go mod tidy   # récupère github.com/PuerkitoBio/goquery
```
# 3️⃣ (Optionnel) Compiler un binaire autonome
```bash
go build -o xaladownloader .
```
Vous pouvez directement lancer le serveur avec go run main.go si vous ne voulez pas compiler.

## ⚙️ Configuration
Le serveur lit (ou crée) un fichier config.json à la racine du projet.
```bash
{
  "base_url": "https://xalaflix.men"
}
```

Modification à chaud : utilisez le formulaire de l’UI ou la requête POST /admin/base-url (voir la section Endpoints API).
Valeur par défaut : si le fichier n’existe pas, le serveur crée automatiquement config.json avec l’URL ci‑dessus.

## ▶️ Lancement
# Depuis le répertoire du projet
```bash
go run main.go
```
Ou, si vous avez compilé :
```bash
./xaladownloader
```
Le serveur écoute par défaut sur http://localhost:8080.

Vous verrez dans le terminal :

```bash
🚀 XalaDownloader démarre sur :8080
```

## 🌐 Endpoints API

| Méthode | URL | Description | Exemple de réponse |
|---------|-----|------------|------------------|
| GET | `/api/search?q=<requête>` | Recherche des médias. Retourne un tableau JSON de Media. | `[{"title":"Avatar","detailUrl":"/shows/details/avatar/123","thumbUrl":"https://.../avatar.jpg"}]` |
| GET | `/api/download?detail=<detailUrl>&title=<titre>` | Télécharge le fichier MP4 correspondant. Renvoie le flux avec Content‑Disposition pour forcer le téléchargement. | Flux binaire (le navigateur propose `Avatar.mp4`). |
| POST | `/admin/base-url` | Met à jour l’URL de base de Xalaflix. Corps JSON : `{ "base_url":"https://nouveau-xalaflix.example" }`. | `204 No Content` si succès. |
| GET | `/` | Sert le répertoire `./ui` contenant l’interface web sombre. | Page HTML |

**Exemple curl pour changer l’URL de base :**
```bash
curl -X POST http://localhost:8080/admin/base-url \
     -H "Content-Type: application/json" \
     -d '{"base_url":"https://nouveau-xalaflix.example"}'
```

## 📜 Licence
Ce projet est publié sous licence MIT. Voir le fichier LICENSE pour les termes complets.

## 📞 Contact
Pseudo : Rajare
GitHub : https://github.com/rajare
N’hésitez pas à ouvrir une issue si vous rencontrez un bug ou avez une suggestion !

Enjoy your downloads! 🚀