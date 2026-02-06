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
search.oninput = async () => {
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
};

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
    if (m.kind === "movie") { 
        download(`/api/download?detail=${m.id}&title=${encodeURIComponent(m.title)}`);
    } else {
        results.scrollIntoView({ behavior: 'smooth' }); 
        results.innerHTML = '<li style="width:100%; text-align:center;">Chargement de la série...</li>';
        try {
            const res = await fetch(`/api/download?detail=${m.id}&infoOnly=true`);
            const sheet = await res.json();
            
            // On s'assure que les données existent avant d'accéder aux index
            if (sheet.data && sheet.data.items.urls.length > 0) {
                const urlTemplate = sheet.data.items.urls[0].url;
                const seasonCount = sheet.data.items.season_count || 1; // Correction nom du champ
                showSeasonSelector(m, urlTemplate, seasonCount);
            }
        } catch (e) {
            results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Erreur de récupération des détails.</li>';
        }
    }
}

function renderMediaList(items) {
    results.innerHTML = '';
    items.forEach(m => {
        const li = document.createElement('li');
        
        if (m.kind === "tv") {
            li.classList.add('is-series');
            const badge = document.createElement('div');
            badge.className = 'badge-series';
            badge.textContent = 'SÉRIE';
            li.appendChild(badge);
        } else {
            const badge = document.createElement('div');
            badge.className = 'badge-film';
            // m.runtime contient déjà "125 min" grâce au backend Go
            badge.textContent = m.runtime;
            li.appendChild(badge); 
        }

        // Correction m.updated au lieu de m.updatedAt
        const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : 'Inconnue';
        
        const overlay = document.createElement('div');
        overlay.className = 'media-info-overlay';
        overlay.innerHTML = `Mis à jour le : <br><strong>${dateStr}</strong>`;
        li.appendChild(overlay);

        li.innerHTML += `
            <img src="${m.thumbUrl}" alt="${m.title}" class="thumb">
            <span class="title">${m.title}</span>
        `;

        li.onclick = () => handleMediaClick(m); 
        results.appendChild(li);
    });
}

/* --------------------------------------------------------------
    Sélecteur de Saisons et Épisodes
-------------------------------------------------------------- */
function showSeasonSelector(media, urlTemplate, seasonCount) {
    results.innerHTML = ''; // Nettoie la grille
    
    // Titre de la série
    const header = document.createElement('h3');
    header.className = 'season-header';
    header.textContent = media.title;
    results.appendChild(header);
    
    // Bouton retour (souvent oublié, mais crucial pour l'UX)
    const backBtn = document.createElement('li');
    backBtn.className = 'season-item';
    backBtn.style.width = "100%"; // On l'étire pour le différencier
    backBtn.innerHTML = '⬅';
    backBtn.onclick = () => search.oninput();
    results.appendChild(backBtn);

    for (let i = 1; i <= seasonCount; i++) {
        const li = document.createElement('li');
        li.className = 'season-item'; // On applique notre nouvelle classe
        
        li.innerHTML = `<span class="season-label">S${i}</span>`;
        
        li.onclick = () => loadEpisodes(media, i, urlTemplate);
        results.appendChild(li);
    }
}

async function loadEpisodes(media, seasonNum, urlTemplate) {
    const sPad = pad(seasonNum);
    results.innerHTML = `<li style="width:100%; text-align:center;">Saison ${sPad}...</li>`;
    
    try {
        const res = await fetch(`/api/episodes?id=${media.id}&num=${seasonNum}`);
        const json = await res.json();
        const episodes = json.data.items.episodes;

        results.innerHTML = `<li style="width:100%; background:var(--accent-primary);" onclick="search.oninput()"><span class="title">⬅ Retour</span></li>`;

        episodes.forEach(ep => {
            const li = document.createElement('li');
            li.style.width = "100%";
            const ePad = pad(ep.episode);

            li.innerHTML = `<span class="title">Épisode ${ePad} : ${ep.name}</span>`;
            
            li.onclick = () => {
                // --- LA LOGIQUE DE REMPLACEMENT DYNAMIQUE ---
                // On remplace les placeholders du template par les valeurs formatées (04, 13, etc.)
                let finalUrl = urlTemplate
                    .replace(/{season_number}/g, sPad)
                    .replace(/{episode_number}/g, ePad);
                
                const finalTitle = `${media.title} S${sPad}E${ePad}`;
                console.log(`Téléchargement de l'épisode : ${finalTitle} depuis l'URL : ${finalUrl}`);
                download(`/api/download?url=${encodeURIComponent(finalUrl)}&title=${encodeURIComponent(finalTitle)}`);
            };
            results.appendChild(li);
        });
    } catch (e) {
        results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Saison non disponible.</li>';
    }
}

function download(apiUrl) {
    bar.hidden = false; bar.value = 0;
    // ... reste de ta fonction download inchangée ...
    const a = document.createElement('a');
    a.href = apiUrl;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
}

window.addEventListener('DOMContentLoaded', loadLastReleases);