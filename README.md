# 📦 XalaDownloader

```
     ██▀███   ▄▄▄      ▄▄▄██▀▀▀▄▄▄       ██▀███  ▓█████ 
    ▓██ ▒ ██▒▒████▄      ▒██  ▒████▄    ▓██ ▒ ██▒▓█   ▀ 
    ▓██ ░▄█ ▒▒██  ▀█▄    ░██  ▒██  ▀█▄  ▓██ ░▄█ ▒▒███   
    ▒██▀▀█▄  ░██▄▄▄▄██▓██▄██▓ ░██▄▄▄▄██ ▒██▀▀█▄  ▒▓█  ▄ 
    ░██▓ ▒██▒ ▓█   ▓██▒▓███▒   ▓█   ▓██▒░██▓ ▒██▒░▒████▒
    ░ ▒▓ ░▒▓░ ▒▒   ▓▒█░▒▓▒▒░   ▒▒   ▓▒█░░ ▒▓ ░▒▓░░░ ▒░ ░
      ░▒ ░ ▒░  ▒   ▒▒ ░▒ ░▒░    ▒   ▒▒ ░  ░▒ ░ ▒░ ░ ░  ░
      ░░   ░   ░   ▒   ░ ░ ░    ░   ▒     ░░   ░    ░   
       ░           ░  ░░   ░        ░  ░   ░        ░  ░
```

Un serveur local en Go conçu pour rechercher, explorer et télécharger du contenu multimédia depuis l'API de streaming de Purstream.

Xalaflix à clairement vendu ses utilisateurs à Purstream, un site moins sécurisé, moins performant, moins fiable et moins complet alors vous n'allez quand même pas payer pour ça, si ?

**Auteur :** Rajare  
**Version :** v1.0.4 (Avril 2026)

---

## 🔧 Prérequis

| Outil | Version minimale |
|-------|----------------|
| Go | 1.22 (ou plus récent) |
| Git | 2.x (pour cloner le dépôt) |
| Internet | Accès à Internet |

---

✨ **Fonctionnalités clés**

 - 🔍 Recherche Intégrée : Trouvez vos films et séries instantanément.

 - 📅 Dernières Sorties : Affichage automatique des 13 derniers ajouts.

 - 🏗️ Gestion des Franchises : Navigation par plateformes (Prime Video, etc.).

 - 📺 Support des Séries : Gestion complète des saisons et épisodes (**Sans abonnement**).

 - 🚀 Mise à jour Auto : Le programme détecte et installe les nouvelles versions au démarrage.

 - 🌐 Détection Dynamique : Utilise purstream.wiki pour trouver automatiquement la base API active.

 - 💻 Interface Web : UI embarquée via go:embed pour une expérience fluide dans le navigateur.

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
go build -trimpath -ldflags="-s -w" -o xaladownloader.exe .
```
Vous pouvez directement lancer le serveur avec go run main.go si vous ne voulez pas compiler.

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

## 📜 Licence
Ce projet est publié sous licence MIT. Voir le fichier LICENSE pour les termes complets.

## 📞 Contact
 - Pseudo : Rajare
 - GitHub : https://github.com/rajare
 - Note : Ce projet est à but éducatif. Respectez les droits d'auteur des contenus que vous visionnez.

Enjoy your downloads! 🚀
