
// This script should be included at the END of the document. 
// For the fastest loading it does not inlude $(document).ready()

// This Document contains a few helper functions for the documentation to display the current version,
// collapse and expand the menu etc.


// Function to make the sticky header possible
function shiftWindow() { 
    scrollBy(0, -70);
    console.log("window shifted")
}

window.addEventListener("hashchange", shiftWindow);

function loadShift() {
    if (window.location.hash) {
        console.log("window has hash");
        shiftWindow();
    }
}

$(window).load(function() {
    loadShift();
});

$(function(){

    // sidebar accordian-ing
    // don't apply on last object (it should be the FAQ) or the first (it should be introduction)

    // define an array to which all opened items should be added
    var openmenus = [];

    var elements = $('.toctree-l2');
    // for (var i = 0; i < elements.length; i += 1) { var current = $(elements[i]); current.children('ul').hide();}


    // set initial collapsed state
    var elements = $('.toctree-l1');
    for (var i = 0; i < elements.length; i += 1) {
        var current = $(elements[i]);
        if (current.hasClass('current')) {
            current.addClass('open');
            currentlink = current.children('a')[0].href;
            openmenus.push(currentlink);

            // do nothing
        } else {
            // collapse children
            current.children('ul').hide();
        }
    }

    // attached handler on click
    // Do not attach to first element or last (intro, faq) so that
    // first and last link directly instead of accordian
    $('.sidebar > ul > li > a').not(':last').not(':first').click(function(){

        var index = $.inArray(this.href, openmenus)

        if (index > -1) {
            console.log(index);
            openmenus.splice(index, 1);


            $(this).parent().children('ul').slideUp(200, function() {
                $(this).parent().removeClass('open'); // toggle after effect
            });
        }
        else {
            openmenus.push(this.href);

            var current = $(this);

            setTimeout(function() {
                // $('.sidebar > ul > li').removeClass('current');
                current.parent().addClass('current').addClass('open'); // toggle before effect
                current.parent().children('ul').hide();
                current.parent().children('ul').slideDown(200);
            }, 100);
        }
        return false;
    });

    // add class to all those which have children
    $('.sidebar > ul > li').not(':last').not(':first').addClass('has-children');


    if (doc_version == "") {
        $('.version-flyer ul').html('<li class="alternative active-slug"><a href="" title="Switch to local">Local</a></li>');
    }

    if (doc_version == "latest") {
        $('.version-flyer .version-note').hide();
    }

    // mark the active documentation in the version widget
    $(".version-flyer a:contains('" + doc_version + "')").parent().addClass('active-slug').setAttribute("title", "Current version");



});