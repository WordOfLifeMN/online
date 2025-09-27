// Get references to the DOM elements
const searchInput = document.getElementById('searchInput');
const resultsMessage = document.getElementById('resultsMessage');
const contentDivs = document.querySelectorAll('.searchable');
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
    resultsMessage.style.opacity = '0';

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
                    const newHtml = originalHtml.replace(regex, `<mark>$&</mark>`);
                    searchable.innerHTML = newHtml;
                    const matchesInDiv = (newHtml.match(/<mark>/g) || []).length;
                    foundCount += matchesInDiv;
                    if (!firstMatch) {
                        firstMatch = seriesDiv.querySelector('mark');
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

// Clear search logic extracted to a function
function clearSearch() {
    searchInput.value = '';
    contentDivs.forEach(div => {
        if (originalContent.has(div)) {
            div.innerHTML = originalContent.get(div);
        }
    });
    document.querySelectorAll('.series-seri').forEach(div => {
        div.style.display = '';
    });
    resultsMessage.innerHTML = '<em>search cleared</em>';
    resultsMessage.style.opacity = '1';
    resultsMessage.style.transition = 'opacity 0.5s';
    setTimeout(() => {
        resultsMessage.style.opacity = '0';
    }, 1500);
}

// Attach event listeners
let searchDebounceTimeout;
searchInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
        if (searchDebounceTimeout) {
            clearTimeout(searchDebounceTimeout);
        }
        searchAndHighlight();
    }
});

// Debounced search: call searchAndHighlight 2s after last input
searchInput.addEventListener('input', function() {
    if (searchInput.value.trim() === '') {
        clearSearch();
        if (searchDebounceTimeout) {
            clearTimeout(searchDebounceTimeout);
        }
        return;
    }
    if (searchDebounceTimeout) {
        clearTimeout(searchDebounceTimeout);
    }
    searchDebounceTimeout = setTimeout(() => {
        searchAndHighlight();
    }, 1500);
});
resultsMessage.style.opacity = '0';


