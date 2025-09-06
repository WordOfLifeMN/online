// Get references to the DOM elements
const searchInput = document.getElementById('searchInput');
const searchButton = document.getElementById('searchButton');
const resultsMessage = document.getElementById('resultsMessage');
const clearSearchButton = document.getElementById('clearSearchButton');
const contentDivs = document.querySelectorAll('p.searchable');
let originalContent = new Map(); // Use a Map to store original content for resetting

// Store the original innerHTML of each content div
contentDivs.forEach((div, index) => {
    originalContent.set(div, div.innerHTML);
});

// Function to remove all existing highlights
const removeHighlights = () => {
    document.querySelectorAll('.search-highlight').forEach(span => {
        // Replace the span with its text content to "un-highlight"
        const parent = span.parentNode;
        if (parent) {
            parent.replaceChild(document.createTextNode(span.textContent), span);
            // Normalize the parent node to merge adjacent text nodes
            parent.normalize();
        }
    });
};

// Main search and highlight function
const searchAndHighlight = () => {
    // Reset all content to its original state
    contentDivs.forEach(div => {
        if (originalContent.has(div)) {
            div.innerHTML = originalContent.get(div);
        }
    });
    resultsMessage.textContent = '';
    clearSearchButton.style.display = 'none';

    const query = searchInput.value.trim();
    const seriesDivs = document.querySelectorAll('.series-seri');

    if (!query) {
        // Show all series divs if query is empty
        seriesDivs.forEach(div => {
            div.style.display = '';
        });
        return;
    }

    let foundCount = 0;
    let firstMatch = null;
    const regex = new RegExp(query, 'gi');

    seriesDivs.forEach(seriesDiv => {
        // Find all searchable elements inside this series div
        const searchables = seriesDiv.querySelectorAll('.searchable');
        let matchFound = false;
        searchables.forEach(searchable => {
            const textContent = searchable.textContent;
            if (textContent.toLowerCase().includes(query.toLowerCase())) {
                matchFound = true;
                // Highlight matches in this searchable
                const originalHtml = originalContent.get(searchable);
                if (originalHtml) {
                    const newHtml = originalHtml.replace(regex, `<span class="search-highlight">$&</span>`);
                    searchable.innerHTML = newHtml;
                    const matchesInDiv = (newHtml.match(/<span class="search-highlight">/g) || []).length;
                    foundCount += matchesInDiv;
                    if (!firstMatch) {
                        firstMatch = seriesDiv.querySelector('.search-highlight');
                    }
                }
            } else {
                // Reset to original if no match
                if (originalContent.has(searchable)) {
                    searchable.innerHTML = originalContent.get(searchable);
                }
            }
        });
        // Show or hide the series div based on match
        seriesDiv.style.display = matchFound ? '' : 'none';
    });

    // Display results and scroll to the first match if found
    if (foundCount > 0) {
        resultsMessage.textContent = `Found ${foundCount} matches.`;
        resultsMessage.style.opacity = '1';
        clearSearchButton.style.display = '';
        setTimeout(() => {
            resultsMessage.style.transition = 'opacity 0.5s';
            resultsMessage.style.opacity = '0';
        }, 2000);
        if (firstMatch) {
            setTimeout(() => {
                firstMatch.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }, 500);
        }
    } else {
        resultsMessage.textContent = 'No matches found.';
        resultsMessage.style.opacity = '1';
        clearSearchButton.style.display = 'none';
        setTimeout(() => {
            resultsMessage.style.transition = 'opacity 0.5s';
            resultsMessage.style.opacity = '0';
        }, 2000);
        // Show all series divs if no matches found
        seriesDivs.forEach(div => {
            div.style.display = '';
        });
    }
};

// Attach event listeners
searchButton.addEventListener('click', searchAndHighlight);
searchInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
        searchAndHighlight();
    }
});

// Clear button logic
clearSearchButton.addEventListener('click', () => {
    searchInput.value = '';
    contentDivs.forEach(div => {
        if (originalContent.has(div)) {
            div.innerHTML = originalContent.get(div);
        }
    });
    document.querySelectorAll('.series-seri').forEach(div => {
        div.style.display = '';
    });
    resultsMessage.textContent = '';
    clearSearchButton.style.display = 'none';
});
