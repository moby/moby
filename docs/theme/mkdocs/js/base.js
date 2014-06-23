$(document).ready(function ()
{

  // Tipue Search activation
  $('#tipue_search_input').tipuesearch({
    'mode': 'json',
    'contentLocation': '/search_content.json'
  });

  prettyPrint();

  // Resizing
  resizeMenuDropdown();
  // checkToScrollTOC();
  $(window).resize(function() {
    if(this.resizeTO)
    {
      clearTimeout(this.resizeTO);
    }
    this.resizeTO = setTimeout(function ()
    {
      resizeMenuDropdown();
      // checkToScrollTOC();
    }, 500);
  });

  /* Follow TOC links (ScrollSpy) */
  $('body').scrollspy({
    target: '#toc_table',
  });

  /* Prevent disabled link clicks */
  $("li.disabled a").click(function ()
  {
    event.preventDefault();
  });

  // Submenu ensured drop-down functionality for desktops & mobiles
  $('.dd_menu').on({
    click: function ()
    {
      $(this).toggleClass('dd_on_hover');
    },
    mouseenter: function ()
    {
      $(this).addClass('dd_on_hover');
    },
    mouseleave: function ()
    {
      $(this).removeClass('dd_on_hover');
    },
  });

});

function resizeMenuDropdown ()
{
  $('.dd_menu > .dd_submenu').css("max-height", ($('body').height() - 160) + 'px');
}

// https://github.com/bigspotteddog/ScrollToFixed
function checkToScrollTOC ()
{
  if ( $(window).width() >= 768 )
  {
    // If TOC is hidden, expand.
    $('#toc_table > #toc_navigation').css("display", "block");
    // Then attach or detach fixed-scroll
    if ( ($('#toc_table').height() + 100) >= $(window).height() )
    {
      $('#toc_table').trigger('detach.ScrollToFixed');
      $('#toc_navigation > li.active').removeClass('active');
    }
    else
    {
      $('#toc_table').scrollToFixed({
        marginTop: $('#nav_menu').height(),
        limit: function () { return $('#footer').offset().top - 450; },
        zIndex: 1,
        minWidth: 768,
        removeOffsets: true,
      });
    }
  }
}

function getCookie(cname) {
  var name = cname + "=";
  var ca = document.cookie.split(';');
  for(var i=0; i<ca.length; i++) {
      var c = ca[i].trim();
      if (c.indexOf(name) == 0) return c.substring(name.length,c.length);
  }
  return "";
}