    <script>
        // Get references to the DOM elements
        const searchInput = document.getElementById('searchInput');
        const searchButton = document.getElementById('searchButton');
        const resultsMessage = document.getElementById('resultsMessage');
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
            // First, reset all content to its original state
            contentDivs.forEach(div => {
                if (originalContent.has(div)) {
                    div.innerHTML = originalContent.get(div);
                }
            });
            resultsMessage.textContent = ''; // Clear previous message

            const query = searchInput.value.trim();
            if (!query) {
                return; // Do nothing if the search query is empty
            }

            let foundCount = 0;
            let firstMatch = null;

            // Use a case-insensitive regular expression for searching
            const regex = new RegExp(query, 'gi');

            contentDivs.forEach(div => {
                const textContent = div.textContent;
                // Check if the query exists in the text content
                if (textContent.toLowerCase().includes(query.toLowerCase())) {
                    // Re-read the original HTML content to avoid issues with previous highlights
                    const originalHtml = originalContent.get(div);
                    const newHtml = originalHtml.replace(regex, `<span class="search-highlight">$&</span>`);
                    div.innerHTML = newHtml;
                    
                    // Count all occurrences in the newly updated HTML
                    const matchesInDiv = (newHtml.match(/<span class="search-highlight">/g) || []).length;
                    foundCount += matchesInDiv;

                    // Find the first highlight span to scroll to
                    if (!firstMatch) {
                        firstMatch = div.querySelector('.search-highlight');
                    }
                }
            });

            // Display results and scroll to the first match if found
            if (foundCount > 0) {
                resultsMessage.textContent = `Found ${foundCount} matches.`;
                if (firstMatch) {
                    setTimeout(() => { // Delay to ensure rendering
                        firstMatch.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    }, 500);
                }
            } else {
                resultsMessage.textContent = 'No matches found.';
            }
        };

        // Attach event listeners
        searchButton.addEventListener('click', searchAndHighlight);
        searchInput.addEventListener('keydown', (event) => {
            if (event.key === 'Enter') {
                searchAndHighlight();
            }
        });

    </script>
