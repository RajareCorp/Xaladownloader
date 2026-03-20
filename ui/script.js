/* --------------------------------------------------------------
    Références DOM et Utilitaires
-------------------------------------------------------------- */
const search   = document.getElementById('search');
const results  = document.getElementById('results');
const bar      = document.getElementById('bar');

const pad = (num) => num.toString().padStart(2, '0');

/* --------------------------------------------------------------
    Logique de Recherche
-------------------------------------------------------------- */
search.oninput =  debounce(async () => {
    if (search.value.length < 2) {
        results.innerHTML = '';
        return;
    }
    try {
        const res = await fetch('/api/search?q=' + encodeURIComponent(search.value));
        const items = await res.json();
        renderMediaList(items);
    } catch (e) {
        results.innerHTML = '<li style="color:#ff6b6b; width:100%; text-align:center;">Erreur recherche.</li>';
    }
});

// Utilitaire de debounce
function debounce(func, timeout = 300) {
    let timer;
    return (...args) => {
        clearTimeout(timer);
        timer = setTimeout(() => { func.apply(this, args); }, timeout);
    };
}

async function loadFranchise(id) {
    // Feedback visuel : on peut changer l'état des boutons ici
    results.innerHTML = '<li style="width:100%; text-align:center; padding: 2rem;">' +
                        '<div class="loading-spinner"></div> Chargement du catalogue...</li>';
    
    try {
        const res = await fetch(`/api/franchise?id=${id}`);
        if (!res.ok) throw new Error();
        const items = await res.json();
        
        // On réutilise ton moteur de rendu ultra-propre
        renderMediaList(items);
        
        // On scroll un peu vers les résultats pour le confort mobile
        window.scrollTo({ top: search.offsetTop - 20, behavior: 'smooth' });
        
    } catch (e) {
        results.innerHTML = '<li style="color:var(--accent-primary); width:100%; text-align:center;">' +
                            'Catalogue indisponible pour le moment.</li>';
    }
}

async function loadLastReleases() {
    const container = document.getElementById('last-releases-list');
    try {
        const res = await fetch('/api/last-releases');
        const items = await res.json();
        
        container.innerHTML = '';
        items.forEach(m => {
            const div = document.createElement('div');
            div.className = 'side-item';
            
            // Correction ici : m.updated au lieu de m.updatedAt
            const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : '';

            div.innerHTML = `
                <img src="${m.thumbUrl}">
                <div class="info">
                    <span class="title">${m.title}</span>
                    <span class="date">${dateStr}</span>
                </div>
            `;

            div.onclick = () => handleMediaClick(m); 
            container.appendChild(div);
        });
    } catch (e) {
        container.innerHTML = '<p style="font-size:0.8rem; color:red;">Erreur de chargement</p>';
    }
}

async function handleMediaClick(m) {
    try {
        const res = await fetch(`/api/download?detail=${m.id}&infoOnly=true`);
        const sheet = await res.json();
        const data = sheet.data.items;

        // On utilise le titre de l'API en priorité pour éviter les 'undefined'
        const mediaTitle = data.title || m.title;

        if (m.kind === "movie") {
            showSourceSelector(mediaTitle, data.urls, m.id);
        } else {
            const organizedData = organizeSeries(sheet);
            
            // On met à jour l'objet media avec le vrai titre pour les fonctions suivantes
            const updatedMedia = { ...m, title: mediaTitle };
            showSeasonSelectorFromData(updatedMedia, organizedData);
        }
    } catch (e) {
        console.error(e);
        results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Erreur de récupération des détails.</li>';
    }
}

function organizeSeries(data) {
    const items = data.data.items;
    const organized = {};

    items.urls.forEach(link => {
        // Regex pour capturer S01E02 ou /S1/E2/
        const sMatch = link.url.match(/S(\d+)/i);
        const eMatch = link.url.match(/E(\d+)/i);

        if (sMatch && eMatch) {
            const sNum = parseInt(sMatch[1]);
            const eNum = parseInt(eMatch[1]);

            if (!organized[sNum]) organized[sNum] = {};
            if (!organized[sNum][eNum]) organized[sNum][eNum] = [];

            organized[sNum][eNum].push({
                url: link.url,
                name: link.name
            });
        }
    });

    return {
        title: items.title,
        seasonsCount: items.seasons,
        content: organized
    };
}

/* --------------------------------------------------------------
    Sélecteur de Sources (Qualités / Formats)
-------------------------------------------------------------- */

async function showSourceSelector(title, urls, mediaId, season = "", episode = "") {
    const modal = document.getElementById('source-modal');
    const list = document.getElementById('source-list');
    document.getElementById('modal-title').textContent = title;

    list.innerHTML = '';
    modal.removeAttribute('hidden'); // Affiche la modal

    urls.forEach((source, index) => {
        const li = document.createElement('li');
        li.className = 'source-item';
        li.id = `source-${index}`;
        
        const isM3U8 = source.url.includes('.m3u8');
        const formatBadge = isM3U8 ? '<span class="badge-m3u8">STREAM</span>' : '<span class="badge-mp4">MP4</span>';
        
        li.innerHTML = `
            <div style="display:flex; align-items:center; gap:10px;">
                <span class="status-dot loading" id="dot-${index}"></span>
                ${formatBadge}
                <span>${source.name}</span>
            </div>
        `;

        li.onclick = () => {
            if (isM3U8) {
                const toast = document.getElementById('m3u8-toast');
                const statusText = document.getElementById('m3u8-status-text');
                
                toast.style.display = 'block';
                
                fetch(`/api/m3u8-download?url=${encodeURIComponent(source.url)}&title=${encodeURIComponent(title)}`);

                // On crée une boucle qui demande l'état au serveur toutes les secondes
                const checker = setInterval(async () => {
                    const res = await fetch(`/api/m3u8-status?title=${encodeURIComponent(title)}`);
                    const data = await res.json();
                    
                    statusText.textContent = data.status;

                    if (data.status === "Terminé !") {
                        clearInterval(checker);
                        setTimeout(() => { toast.style.display = 'none'; }, 5000); // Cache après 5s
                    }
                }, 1000);
            } else {
                // Téléchargement direct MP4 classique
                const downloadUrl = `/api/download?detail=${mediaId}&selectedUrl=${encodeURIComponent(source.url)}&title=${encodeURIComponent(title)}`;
                download(downloadUrl);
            }
            closeModal();
        };

        list.appendChild(li);

        // --- TEST DE VALIDITÉ EN ARRIÈRE-PLAN ---
        checkLinkStatus(source.url, index);
    });
}

async function checkLinkStatus(url, index) {
    const dot = document.getElementById(`dot-${index}`);
    try {
        const res = await fetch(`/api/check-url?url=${encodeURIComponent(url)}`);
        const data = await res.json();
        
        dot.classList.remove('loading');
        if (data.status === "ok") {
            dot.classList.add('online');
        } else {
            dot.classList.add('offline');
            document.getElementById(`source-${index}`).style.opacity = "0.5";
        }
    } catch (e) {
        dot.classList.add('offline');
    }
}

function closeModal() {
    const modal = document.getElementById('source-modal');
    modal.setAttribute('hidden', 'true');
    // SUPPRESSION de search.oninput() pour garder la liste intacte derrière !
}

function renderMediaList(items) {
    results.innerHTML = items.map(m => {
        const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : 'Inconnue';
        const badge = m.kind === "tv" 
            ? '<div class="badge-series">SÉRIE</div>' 
            : `<div class="badge-film">${m.runtime}</div>`;

        return `
            <li class="${m.kind === 'tv' ? 'is-series' : ''}" data-id="${m.id}">
                ${badge}
                <div class="media-info-overlay">Mis à jour le : <br><strong>${dateStr}</strong></div>
                <img src="${m.thumbUrl}" alt="${m.title}" class="thumb">
                <span class="title">${m.title}</span>
            </li>
        `;
    }).join('');

    // On attache les événements après le rendu global
    Array.from(results.children).forEach((li, index) => {
        li.onclick = () => handleMediaClick(items[index]);
    });
}


/* --------------------------------------------------------------
    Sélecteur de Saisons et Épisodes
-------------------------------------------------------------- */

function showSeasonSelectorFromData(media, organizedData) {
    results.innerHTML = `<h3 class="season-header">${organizedData.title}</h3>`;
    
    // Bouton retour à la recherche
    const backBtn = document.createElement('li');
    backBtn.className = 'season-item';
    backBtn.style.width = "100%";
    backBtn.innerHTML = '⬅ Retour';
    backBtn.onclick = () => search.oninput();
    results.appendChild(backBtn);

    // On affiche les saisons trouvées par le Regex
    const seasons = Object.keys(organizedData.content).sort((a, b) => a - b);
    
    seasons.forEach(sNum => {
        const li = document.createElement('li');
        li.className = 'season-item';
        li.innerHTML = `<span class="season-label">Saison ${sNum}</span>`;
        li.onclick = () => renderEpisodesFromData(media, sNum, organizedData.content[sNum]);
        results.appendChild(li);
    });
}

function renderEpisodesFromData(media, sNum, episodesMap) {
    // Correction : On repasse l'objet 'media' complet au clic sur le retour
    const backLi = document.createElement('li');
    backLi.style.width = "100%";
    backLi.style.background = "var(--accent-primary)";
    backLi.innerHTML = `<span class="title">⬅ Retour aux Saisons</span>`;
    backLi.onclick = () => handleMediaClick(media); 
    
    results.innerHTML = '';
    results.appendChild(backLi);

    const episodes = Object.keys(episodesMap).sort((a, b) => a - b);

    episodes.forEach(eNum => {
        const li = document.createElement('li');
        li.style.width = "100%";
        const sources = episodesMap[eNum];
        
        li.innerHTML = `<span class="title">Épisode ${pad(eNum)}</span>`;
        li.onclick = () => {
            // Ici media.title sera bien défini
            const finalTitle = `${media.title} S${pad(sNum)}E${pad(eNum)}`;
            showSourceSelector(finalTitle, sources, media.id);
        };
        results.appendChild(li);
    });
}

function download(apiUrl) {
    // 1. Préparation de l'UI
    bar.hidden = false;
    bar.value = 0;
    
    // On crée la requête
    const xhr = new XMLHttpRequest();
    xhr.open('GET', apiUrl, true);
    xhr.responseType = 'blob'; // On attend un fichier binaire

    // 2. Suivi de la progression
    xhr.onprogress = (event) => {
        if (event.lengthComputable) {
            const percentComplete = event.loaded / event.total;
            bar.value = percentComplete; // Met à jour la barre (0 à 1)
        }
    };

    // 3. Une fois le téléchargement terminé (dans la RAM du navigateur)
    xhr.onload = () => {
        if (xhr.status === 200) {
            // Création d'un lien temporaire pour déclencher l'enregistrement sur le disque
            const blob = xhr.response;
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            
            // On récupère le nom du fichier via l'en-tête Content-Disposition si possible
            // Sinon on utilise un nom générique
            a.href = url;
            a.download = ""; // Le navigateur utilisera le nom envoyé par Go
            document.body.appendChild(a);
            a.click();
            
            // Nettoyage
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
            
            // Feedback final
            setTimeout(() => { bar.hidden = true; }, 2000);
        } else {
            alert("Erreur lors du téléchargement (Proxy).");
            bar.hidden = true;
        }
    };

    xhr.onerror = () => {
        alert("Erreur réseau.");
        bar.hidden = true;
    };

    xhr.send();
}

window.addEventListener('DOMContentLoaded', loadLastReleases);